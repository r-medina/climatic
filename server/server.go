package server

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/jobcoin"

	"github.com/satori/go.uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
)

func init() { rand.Seed(time.Now().UTC().UnixNano()) }

// Mixer implements the mixer interface and is a Jobcoin mixer.
type Mixer struct {
	addr string

	jcClient jobcoin.Client
	ds       Datastore

	fee           float64
	lastSeenTxIdx int

	outstanding map[string]*mix
	mtx         sync.Mutex

	pollCfg PollConfig
	mixCfg  MixConfig

	log grpclog.Logger
}

var _ climatic.MixerServer = (*Mixer)(nil)

// NewMixer instantiates a new Mixer.
func NewMixer(opts ...Option) (*Mixer, error) {
	addr, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	mxr := &Mixer{
		jcClient:    jobcoin.NewClimaticClient(),
		ds:          newMemDS(),
		addr:        addr.String(),
		outstanding: map[string]*mix{},
		pollCfg:     DefaultPollConfig,
		mixCfg:      DefaultMixConfig,
		log:         log.New(os.Stderr, "", log.LstdFlags),
	}

	for _, opt := range opts {
		opt(mxr)
	}

	return mxr, nil
}

// Option customizes a Mixer.
type Option func(*Mixer)

// WithJobcoinClient allows caller to specify a jobcoin client.
func WithJobcoinClient(jcClient jobcoin.Client) Option {
	return func(mxr *Mixer) {
		mxr.jcClient = jcClient
	}
}

// WithAddress allows you to specify the address of the mixer.
func WithAddress(addr string) Option {
	return func(mxr *Mixer) {
		mxr.addr = addr
	}
}

// WithFee determines the fee that the mixer will take for mixing
func WithFee(fee float64) Option {
	return func(mxr *Mixer) {
		mxr.fee = fee
	}
}

// WithPollConfig specifies the polling configuration. The values are made valid
// silently.
func WithPollConfig(pollCfg PollConfig) Option {
	return func(mxr *Mixer) {
		pollCfg.makeValid()
		mxr.pollCfg = pollCfg
	}
}

// WithMixConfig specifies the mixing configuration. The values are made alid
// silently.
func WithMixConfig(mixCfg MixConfig) Option {
	return func(mxr *Mixer) {
		mixCfg.makeValid()
		mxr.mixCfg = mixCfg
	}
}

// WithLogger specifies the logger.
func WithLogger(log grpclog.Logger) Option {
	return func(mxr *Mixer) {
		mxr.log = log
	}
}

// Register allows the caller to register their addresses and receive a jobcoin
// address to make deposits.
func (mxr *Mixer) Register(
	ctx context.Context, req *climatic.RegisterRequest,
) (*climatic.RegisterResponse, error) {
	l := mxr.log
	l.Printf("Register called: %v", *req)

	depositAddr, err := uuid.NewV4()
	if err != nil {
		l.Printf("could not make deposit addr: %v", err)
		return nil, grpc.Errorf(codes.Internal, "could not generate deposit address")
	}
	l.Printf("deposit addr: %v", depositAddr)

	if len(req.Addresses) == 0 {
		l.Printf("addresses empty")
		return nil, grpc.Errorf(codes.InvalidArgument, "addresses invalid")
	}

	if err := mxr.ds.Register(depositAddr.String(), req.Addresses); err != nil {
		l.Printf("could not register deposit addr: %v", err)
		return nil, grpc.Errorf(codes.Internal, "could not register addresses")
	}

	return &climatic.RegisterResponse{Address: depositAddr.String()}, nil
}

// Start starts the threads that poll jobcoin and deposits the coins.
func (mxr *Mixer) Start() error {
	l := mxr.log

	go func() {
		for {
			l.Printf("running poll")
			if err := mxr.poll(); err != nil {
				l.Printf("poll failed: %v", err)
			}

			<-time.After(mxr.pollCfg.delay())
		}
	}()

	for {
		l.Printf("running mix")
		if err := mxr.mix(); err != nil {
			l.Printf("mix failed: %v", err)
		}

		<-time.After(mxr.mixCfg.delay())
	}
}

func (mxr *Mixer) poll() error {
	l := mxr.log

	txs, err := mxr.jcClient.GetTransactions()
	if err != nil {
		return err
	}

	// ignore transactions we've seen
	txs = txs[mxr.lastSeenTxIdx:]
	// set lastSeenTxIdx to appropriate value
	mxr.lastSeenTxIdx += len(txs)

	mixReqs := []mixRequest{}
	for _, tx := range txs {
		usrAddrs, _ := mxr.ds.UserAddresses(tx.ToAddress)
		if len(usrAddrs) == 0 {
			continue
		}

		// for logging
		buf, _ := json.Marshal(tx)
		l.Printf("found transaction to mix: %v", string(buf))

		mixReqs = append(mixReqs, mixRequest{tx: tx, usrAddrs: usrAddrs})
	}

	// add the new requested mixes after a delay
	go func() {
		<-time.After(mxr.mixCfg.InitialDelay)

		mxr.mtx.Lock()
		defer mxr.mtx.Unlock()

		for _, mixReq := range mixReqs {
			m, ok := mxr.outstanding[mixReq.tx.ToAddress]
			if ok {
				m.inputTxs = append(m.inputTxs, mixReq.tx)
				_, err := mxr.updateRemaining(mixReq.tx.ToAddress, &m.remaining)
				if err != nil {
					l.Printf("failed to update remaining: %v", err)
				}
				continue
			}

			mxr.outstanding[mixReq.tx.ToAddress] = &mix{
				inputTxs:  []*jobcoin.Transaction{mixReq.tx},
				mixed:     []*jobcoin.Transaction{},
				usrAddrs:  mixReq.usrAddrs,
				remaining: mixReq.tx.Amount,
			}
		}
	}()

	return nil
}

func (mxr *Mixer) mix() error {
	l := mxr.log

	// for formatting floats as strings as precisely as possible

	mxr.mtx.Lock()
	defer mxr.mtx.Unlock()

	// if there are no outstanding things to be mixed, exit
	if len(mxr.outstanding) < 1 {
		l.Printf("nothing to mix")
		return nil
	}
	// select random mix request
	i := rand.Intn(len(mxr.outstanding))
	var addr string
	for addr = range mxr.outstanding {
		if i == 0 {
			break
		}
		i--
	}
	m := mxr.outstanding[addr]
	// if no user addresses regitered, exit
	if len(m.usrAddrs) < 1 {
		return nil
	}

	// TODO: fee larger than amount

	// collect fee
	if mxr.fee == 0 {
		m.feePaid = true
	}
	if !m.feePaid {
		fee := climatic.Ftos(mxr.fee)
		// send fee from deposit address to mixer address
		err := mxr.jcClient.PostTransaction(addr, mxr.addr, fee)
		if err != nil {
			l.Printf("error collecting fee: %v", err)
		} else {
			m.feePaid = true
			_, err := mxr.updateRemaining(addr, &m.remaining)
			if err != nil {
				return err
			}
		}
	}

	remaining, err := strconv.ParseFloat(m.remaining, 64)
	if err != nil {
		return err
	}
	amt := mxr.mixCfg.amount()
	if amt > remaining {
		amt = remaining
	}
	if amt == 0 {
		return nil
	}

	usrAddr := m.usrAddrs[rand.Intn(len(m.usrAddrs))]
	l.Printf("mixing from %v to %v with amount %f", addr, usrAddr, amt)
	if amt == remaining {
		err = mxr.jcClient.PostTransaction(addr, usrAddr, m.remaining)
	} else {
		err = mxr.jcClient.PostTransaction(addr, usrAddr, climatic.Ftos(amt))
	}
	if err != nil {
		return err
	}
	remaining, err = mxr.updateRemaining(addr, &m.remaining)
	if err != nil {
		return err
	}
	if remaining == 0 {
		// TODO: save in another data structure rather than delete
		l.Printf("done mixing %v: %+v", addr, m.inputTxs)
		delete(mxr.outstanding, addr)
	}

	return nil
}

// updateRemaining updates the remaining amount and returns a float64 of it.
func (mxr *Mixer) updateRemaining(addr string, remaining *string) (float64, error) {
	addrInfo, err := mxr.jcClient.GetAddressInfo(addr)
	if err != nil {
		return -0, err
	}
	*remaining = addrInfo.Balance
	out, err := strconv.ParseFloat(addrInfo.Balance, 64)
	if err != nil {
		return 0, err
	}

	return out, nil
}

type mix struct {
	inputTxs  []*jobcoin.Transaction
	mixed     []*jobcoin.Transaction
	usrAddrs  []string
	remaining string
	feePaid   bool
}

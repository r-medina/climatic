package server

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"os"
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

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			l.Printf("running poll")
			if err := mxr.poll(); err != nil {
				l.Printf("poll failed: %v", err)
			}

			<-time.After(mxr.pollCfg.delay())
		}
	}()

	go func() {
		defer wg.Done()
		for {
			l.Printf("running mix")
			if err := mxr.mix(); err != nil {
				l.Printf("mix failed: %v", err)
			}

			<-time.After(mxr.mixCfg.delay())
		}
	}()

	wg.Wait()

	return nil
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
		mxr.makeMix(mixReqs)
	}()

	return nil
}

// makeMix takes mix requests and adds them to the queue of Jobcoins to be mixed.
// This function was broken out for testing purposes.
func (mxr *Mixer) makeMix(mixReqs []mixRequest) {
	mxr.mtx.Lock()
	defer mxr.mtx.Unlock()

	for _, mixReq := range mixReqs {
		m, ok := mxr.outstanding[mixReq.tx.ToAddress]
		if ok {
			m.remaining += mixReq.tx.Amount
			continue
		}

		mxr.outstanding[mixReq.tx.ToAddress] = &mix{
			usrAddrs:  mixReq.usrAddrs,
			remaining: mixReq.tx.Amount,
		}
	}
}

// mix does the mixing. This function assumes that no other rthreads are
// spending Jobcoins in the deposit addresses mxr knows about.
func (mxr *Mixer) mix() error {
	l := mxr.log

	mxr.mtx.Lock()
	defer mxr.mtx.Unlock()

	//
	// The first few blocks are for selecting a mix request to send part of.
	//

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
	// If no user addresses regitered, exit. This case should never get hit,
	// but the state is possible.
	if len(m.usrAddrs) < 1 {
		return nil
	}

	// Tick a random user address to send the amount to. This is selected
	// here so that the rest of the function is deterministic.
	usrAddr := m.usrAddrs[rand.Intn(len(m.usrAddrs))]

	//
	// The rest of the function collects the fee and sends mixed Jobcoins.
	//

	l.Printf("mixing %v", addr)

	// prevents a class of rounding error
	defer func() {
		del, err := mxr.updateRemaining(addr, m)
		if err != nil {
			l.Printf("failed to update remaining: %v", err)
		}
		if del {
			delete(mxr.outstanding, addr)
		}
	}()

	// collect fee
	if mxr.fee == 0 {
		m.feePaid = true
	}
	if !m.feePaid {
		l.Printf("collecting fee")
		if err := mxr.getFee(m, addr); err != nil {
			return err
		}
	}

	return mxr.sendMix(m, addr, usrAddr)
}

// updateRemaining gets the API's view of the remaining balance and updates
// it. It also returns of the mix request should be deleted.
func (mxr *Mixer) updateRemaining(addr string, m *mix) (bool, error) {
	l := mxr.log

	remaining, err := mxr.getRemaining(addr)
	if err != nil {
		l.Printf("failed to get remaining: %v", err)
		return false, err
	}
	if remaining == 0 {
		l.Printf("done mixing %v", addr)
		return true, nil
	}

	return false, nil
}

func (mxr *Mixer) getFee(m *mix, addr string) error {
	l := mxr.log

	// if the fee is larger than the amount to be mixed, just use the remaining amount
	fee := mxr.fee
	if mxr.fee > m.remaining {
		fee = m.remaining
		l.Printf("reduced fee: %f", fee)
	}

	// send fee from deposit address to mixer address
	err := mxr.jcClient.PostTransaction(addr, mxr.addr, fee)
	if err != nil {
		return err
	}

	m.feePaid = true
	m.remaining -= fee

	return nil
}

func (mxr *Mixer) sendMix(m *mix, addr, usrAddr string) error {
	l := mxr.log

	// if the amount is greater than the total remaining, only mix the remaining
	amt := mxr.mixCfg.amount()
	if amt > m.remaining {
		amt = m.remaining
	}

	if amt != 0 {
		// There's a small chance that the rest of this function will
		// fail due to a discrepancy between our internal accounting and
		// the balance on the server. The next time this address is
		// mixed, it will work due to the updated remaining amount.

		l.Printf("mixing from %v to %v with amount %s", addr, usrAddr, climatic.Ftos(amt))
		err := mxr.jcClient.PostTransaction(addr, usrAddr, amt)
		if err != nil {
			return err
		}
		m.remaining -= amt
	}

	return nil
}

func (mxr *Mixer) getRemaining(addr string) (float64, error) {
	addrInfo, err := mxr.jcClient.GetAddressInfo(addr)
	if err != nil {
		return 0, err
	}
	return addrInfo.Balance, nil

}

type mix struct {
	usrAddrs  []string
	remaining float64
	feePaid   bool
}

type mixRequest struct {
	tx       *jobcoin.Transaction
	usrAddrs []string
}

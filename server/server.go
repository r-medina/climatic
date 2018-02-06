package server

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/jobcoin"

	"github.com/satori/go.uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

// Register allows the caller to register their addresses and receive a jobcoin
// address to make deposits.
func (mxr *Mixer) Register(
	ctx context.Context, req *climatic.RegisterRequest,
) (*climatic.RegisterResponse, error) {
	depositAddr, err := uuid.NewV4()
	if err != nil {
		// TODO: log
		return nil, grpc.Errorf(codes.Internal, "could not generate deposit address")
	}

	if len(req.Addresses) == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "addresses invalid")
	}

	if err := mxr.ds.Register(depositAddr.String(), req.Addresses); err != nil {
		// TODO: log
		return nil, grpc.Errorf(codes.Internal, "could not register addresses")
	}

	return &climatic.RegisterResponse{Address: depositAddr.String()}, nil
}

// Start starts the threads that poll jobcoin and deposits the coins.
func (mxr *Mixer) Start() error {
	go func() {
		for {
			if err := mxr.poll(); err != nil {
				// TODO: log
			}

			<-time.After(mxr.pollCfg.delay())
		}
	}()

	for {
		if err := mxr.mix(); err != nil {
			// TODO: log
		}

		<-time.After(mxr.mixCfg.delay())
	}
}

func (mxr *Mixer) poll() error {
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
		usrAddrs, err := mxr.ds.UserAddresses(tx.ToAddress)
		if err != nil {
			// TODO: log
			continue
		}

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
				m.remaining += mixReq.tx.Amount
				continue
			}

			mxr.outstanding[mixReq.tx.ToAddress] = &mix{
				inputTxs:  []*jobcoin.Transaction{mixReq.tx},
				mixed:     []*jobcoin.Transaction{},
				usrAddrs:  mixReq.usrAddrs,
				remaining: mixReq.tx.Amount - mxr.fee,
			}
		}
	}()

	return nil
}

func (mxr *Mixer) mix() error {
	mxr.mtx.Lock()
	defer mxr.mtx.Unlock()

	if len(mxr.outstanding) < 1 {
		return nil
	}
	i := rand.Intn(len(mxr.outstanding))
	var addr string
	for addr = range mxr.outstanding {
		if i == 0 {
			break
		}
		i--
	}
	m := mxr.outstanding[addr]
	fromAddr := m.inputTxs[0].FromAddress

	amt := mxr.mixCfg.amount()
	amt = math.Min(m.remaining, amt)

	if len(m.usrAddrs) < 1 {
		return nil
	}
	i = rand.Intn(len(m.usrAddrs))
	err := mxr.jcClient.PostTransaction(fromAddr, m.usrAddrs[i], amt)
	if err != nil {
		return err
	}
	m.remaining -= amt
	if m.remaining == 0 {
		delete(mxr.outstanding, addr)
	}

	// collect fee
	if !m.feePaid {
		err := mxr.jcClient.PostTransaction(fromAddr, mxr.addr, mxr.fee)
		if err != nil {
			// TODO: log
		} else {
			m.feePaid = true
		}
	}

	return nil
}

type mix struct {
	inputTxs  []*jobcoin.Transaction
	mixed     []*jobcoin.Transaction
	usrAddrs  []string
	remaining float64
	feePaid   bool
}

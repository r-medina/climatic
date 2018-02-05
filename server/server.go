package server

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/jobcoin"

	"github.com/satori/go.uuid"
)

func init() { rand.Seed(time.Now().UTC().UnixNano()) }

// Mixer implements the mixer interface and is a Jobcoin mixer.
type Mixer struct {
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
func NewMixer(opts ...Option) *Mixer {
	mxr := &Mixer{
		ds:          newMemDS(),
		outstanding: map[string]*mix{},
		pollCfg:     DefaultPollConfig,
		mixCfg:      DefaultMixConfig,
	}

	for _, opt := range opts {
		opt(mxr)
	}

	return mxr
}

// Option customizes a Mixer.
type Option func(*Mixer)

// WithJobcoinClient allows caller to specify a jobcoin client.
func WithJobcoinClient(jcClient jobcoin.Client) Option {
	return func(mxr *Mixer) {
		mxr.jcClient = jcClient
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
	// TODO: take fee on first

	mxr.mtx.Lock()
	defer mxr.mtx.Unlock()

	i := rand.Intn(len(mxr.outstanding))
	var addr string
	for addr = range mxr.outstanding {
		if i == 0 {
			break
		}
		i--
	}
	m := mxr.outstanding[addr]

	if len(m.mixed) == 0 {

	}

	return nil
}

// Datastore contains the functions necessary from a datastore for the Mixer
type Datastore interface {
	Register(depositAddr string, usrAddrs []string) error
	DepositAddresses() ([]string, error)
	UserAddresses(depositAddr string) ([]string, error)
}

// memDS implements Datastore in memory.
type memDS struct {
	addrs map[string][]string
	mtx   sync.RWMutex
}

var _ Datastore = (*memDS)(nil)

func newMemDS() *memDS {
	return &memDS{addrs: map[string][]string{}}
}

func (ds *memDS) Register(depositAddr string, usrAddrs []string) error {
	ds.mtx.Lock()
	defer ds.mtx.Unlock()

	ds.addrs[depositAddr] = usrAddrs

	return nil
}

func (ds *memDS) DepositAddresses() ([]string, error) {
	depositAddrs := []string{}

	ds.mtx.RLock()
	defer ds.mtx.RUnlock()

	for addr := range ds.addrs {
		depositAddrs = append(depositAddrs, addr)
	}

	return depositAddrs, nil
}

func (ds *memDS) UserAddresses(depositAddr string) ([]string, error) {
	ds.mtx.RLock()
	defer ds.mtx.RUnlock()

	usrAddrs := ds.addrs[depositAddr]

	return usrAddrs, nil
}

type mix struct {
	inputTxs  []*jobcoin.Transaction
	mixed     []*jobcoin.Transaction
	usrAddrs  []string
	remaining float64
	feePaid   bool
}

// PollConfig configures the polling loop in the Mixer.
type PollConfig struct {
	MeanDelay   time.Duration
	StdDevDelay time.Duration
	MinDelay    time.Duration
	MaxDelay    time.Duration
}

func (pollCfg *PollConfig) makeValid() {
	if pollCfg.MeanDelay-pollCfg.StdDevDelay < 0 {
		pollCfg.StdDevDelay = pollCfg.MeanDelay / 2
	}

	if pollCfg.MinDelay < 0 {
		pollCfg.MinDelay = 0
	}

	if pollCfg.MaxDelay < pollCfg.MeanDelay {
		pollCfg.MaxDelay = pollCfg.MeanDelay
	}
}

// delay determines the polling interval by sampling from a normal distribution
// with the configured mean interval and standard deviation.
func (pollCfg PollConfig) delay() time.Duration {
	return delay(pollCfg.MeanDelay, pollCfg.StdDevDelay, pollCfg.MinDelay, pollCfg.MaxDelay)
}

// DefaultPollConfig is the default polling configuration.
var DefaultPollConfig = PollConfig{
	MeanDelay:   10 * time.Second,
	StdDevDelay: 3 * time.Second,
	MinDelay:    2 * time.Second,
	MaxDelay:    20 * time.Second,
}

// MixConfig configures the mixer.
type MixConfig struct {
	MeanDelay    time.Duration
	StdDevDelay  time.Duration
	MinDelay     time.Duration
	MaxDelay     time.Duration
	InitialDelay time.Duration

	MeanAmount   float64
	StdDevAmount float64
	MinAmount    float64
	MaxAmount    float64
}

func (mixCfg *MixConfig) makeValid() {
	if mixCfg.MeanDelay-mixCfg.StdDevDelay < 0 {
		mixCfg.StdDevDelay = mixCfg.MeanDelay / 2
	}

	if mixCfg.MinDelay < 0 {
		mixCfg.MinDelay = 0
	}

	if mixCfg.MaxDelay < mixCfg.MeanDelay {
		mixCfg.MaxDelay = mixCfg.MeanDelay
	}

	if mixCfg.MeanAmount-mixCfg.StdDevAmount < 0 {
		mixCfg.StdDevAmount = mixCfg.MeanAmount / 2
	}

	if mixCfg.MinAmount < 0 {
		mixCfg.MinAmount = 1.
	}

	if mixCfg.MaxAmount < mixCfg.MeanAmount {
		mixCfg.MaxAmount = mixCfg.MeanAmount
	}
}

func (mixCfg MixConfig) delay() time.Duration {
	return delay(mixCfg.MeanDelay, mixCfg.StdDevDelay, mixCfg.MinDelay, mixCfg.MaxDelay)
}

func (mixCfg MixConfig) amount() float64 {
	return norm(
		mixCfg.MeanAmount,
		mixCfg.StdDevAmount,
		mixCfg.MinAmount,
		mixCfg.MaxAmount,
	)
}

// DefaultMixConfig is the default mixing configuration.
var DefaultMixConfig = MixConfig{
	MeanDelay:    1 * time.Second,
	StdDevDelay:  250 * time.Millisecond,
	MinDelay:     50 * time.Millisecond,
	MaxDelay:     3 * time.Second,
	InitialDelay: 3 * time.Minute,

	MeanAmount:   10.,
	StdDevAmount: 8.,
	MinAmount:    5.,
	MaxAmount:    100.,
}

func delay(delay, stdDev, min, max time.Duration) time.Duration {
	return time.Duration(norm(float64(delay), float64(stdDev), float64(min), float64(max)))
}

func norm(mean, stdDev, min, max float64) float64 {
	n := rand.NormFloat64()*stdDev + mean
	n = math.Min(min, n)
	n = math.Max(max, n)

	return n
}

type mixRequest struct {
	tx       *jobcoin.Transaction
	usrAddrs []string
}

package server

import (
	"math"
	"math/rand"
	"time"

	"github.com/r-medina/climatic/jobcoin"
)

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

// TODO: log

package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/server"

	"google.golang.org/grpc"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var config struct {
	addr      string
	fee       float64
	pollCfg   server.PollConfig
	mixCfg    server.MixConfig
	pprofAddr string
}

var (
	app = kingpin.New("climasrv", "climatic server").
		PreAction(startPprof).Action(runServer).DefaultEnvars()
)

func init() {
	app.Flag("fee", "fee to charge people using the service").FloatVar(&config.fee)

	app.Flag("poll-delay", "mean of delay between polls to jobcoin API").
		DurationVar(&config.pollCfg.MeanDelay)
	app.Flag("poll-dev", "the standard deviation of time between polls to jobcoin API").
		DurationVar(&config.pollCfg.StdDevDelay)
	app.Flag("poll-min-delay", "the minimum delay between polling").
		DurationVar(&config.pollCfg.MinDelay)
	app.Flag("poll-max-delay", "the maximum delay between polling").
		DurationVar(&config.pollCfg.MaxDelay)

	app.Flag("mix-delay", "mean of delay between times that jobcoins are mixed").
		DurationVar(&config.mixCfg.MeanDelay)
	app.Flag("mix-dev", "the standard deviation of time between mixes").
		DurationVar(&config.mixCfg.StdDevDelay)
	app.Flag("mix-min-delay", "the minimum delay between mixing").
		DurationVar(&config.mixCfg.MinDelay)
	app.Flag("mix-max-delay", "the maximum delay between mixing").
		DurationVar(&config.mixCfg.MaxDelay)
	app.Flag(
		"mix-initial-delay",
		"the time between receiving a mix request and first time it is eligible for mixing",
	).DurationVar(&config.mixCfg.MaxDelay)
	app.Flag("mix-amount", "mean of amount of jobcoins sent per transaction").
		FloatVar(&config.mixCfg.MeanAmount)
	app.Flag("mix-dev-amount", "the standard deviation of jobcoins sent per transaction").
		FloatVar(&config.mixCfg.StdDevAmount)
	app.Flag("mix-min-amount", "the minimum amount of jobcoins sent").
		FloatVar(&config.mixCfg.MinAmount)
	app.Flag("mix-max-amount", "the maximum amount of jobcoins sent").
		FloatVar(&config.mixCfg.MaxAmount)

	app.Flag("pprof-addr", "address for running pprof tools").StringVar(&config.pprofAddr)

}

func main() {
	if _, err := app.Parse(os.Args[1:]); err != nil {
		log.Fatalf("command line parsing failed: %v", err)
	}
}

func runServer(_ *kingpin.ParseContext) error {
	opts := []server.Option{}

	opts = append(opts, server.WithFee(config.fee))
	if cfg := config.pollCfg; cfg != (server.PollConfig{}) {
		opts = append(opts, server.WithPollConfig(cfg))
	}
	if cfg := config.mixCfg; cfg != (server.MixConfig{}) {
		opts = append(opts, server.WithMixConfig(cfg))
	}

	mxr, err := server.NewMixer(opts...)
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", config.addr)
	if err != nil {
		return err
	}

	// TODO: log interceptor
	grpcSrv := grpc.NewServer()

	climatic.RegisterMixerServer(grpcSrv, mxr)

	go mxr.Start()
	log.Printf("listening at %v", lis.Addr())
	_ = grpcSrv.Serve(lis)

	return nil
}

func startPprof(_ *kingpin.ParseContext) error {
	if config.pprofAddr == "" {
		return nil
	}

	log.Printf("running pprof server on %s", config.pprofAddr)
	go func() {
		log.Println(http.ListenAndServe(config.pprofAddr, nil))
	}()

	return nil
}

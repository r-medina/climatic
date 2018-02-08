// TODO: log

package main

import (
	"fmt"
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
	tcpAddr   *net.TCPAddr
	fee       float64
	pollCfg   server.PollConfig
	mixCfg    server.MixConfig
	pprofAddr *net.TCPAddr
}

var (
	app = kingpin.New("climasrv", "climatic server").
		PreAction(startPprof).Action(runServer).DefaultEnvars()

	l = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
)

func init() {
	app.Flag("tcp-addr", "address for TCP listener").Default("").TCPVar(&config.tcpAddr)
	app.Flag("fee", "fee to charge people using the service").FloatVar(&config.fee)

	app.Flag("poll-delay", "mean of delay between polls to jobcoin API").
		Default(str(server.DefaultPollConfig.MeanDelay)).
		DurationVar(&config.pollCfg.MeanDelay)
	app.Flag("poll-dev", "the standard deviation of time between polls to jobcoin API").
		Default(str(server.DefaultPollConfig.StdDevDelay)).
		DurationVar(&config.pollCfg.StdDevDelay)
	app.Flag("poll-min-delay", "the minimum delay between polling").
		Default(str(server.DefaultPollConfig.MinDelay)).
		DurationVar(&config.pollCfg.MinDelay)
	app.Flag("poll-max-delay", "the maximum delay between polling").
		Default(str(server.DefaultPollConfig.MaxDelay)).
		DurationVar(&config.pollCfg.MaxDelay)

	app.Flag("mix-delay", "mean of delay between times that jobcoins are mixed").
		Default(str(server.DefaultMixConfig.MeanDelay)).
		DurationVar(&config.mixCfg.MeanDelay)
	app.Flag("mix-dev", "the standard deviation of time between mixes").
		Default(str(server.DefaultMixConfig.StdDevDelay)).
		DurationVar(&config.mixCfg.StdDevDelay)
	app.Flag("mix-min-delay", "the minimum delay between mixing").
		Default(str(server.DefaultMixConfig.MinDelay)).
		DurationVar(&config.mixCfg.MinDelay)
	app.Flag("mix-max-delay", "the maximum delay between mixing").
		Default(str(server.DefaultMixConfig.MaxDelay)).
		DurationVar(&config.mixCfg.MaxDelay)
	app.Flag(
		"mix-initial-delay",
		"the time between receiving a mix request and first time it is eligible for mixing",
	).Default(str(server.DefaultMixConfig.InitialDelay)).
		DurationVar(&config.mixCfg.InitialDelay)
	app.Flag("mix-amount", "mean of amount of jobcoins sent per transaction").
		Default(str(server.DefaultMixConfig.MeanAmount)).
		FloatVar(&config.mixCfg.MeanAmount)
	app.Flag("mix-dev-amount", "the standard deviation of jobcoins sent per transaction").
		Default(str(server.DefaultMixConfig.StdDevAmount)).
		FloatVar(&config.mixCfg.StdDevAmount)
	app.Flag("mix-min-amount", "the minimum amount of jobcoins sent").
		Default(str(server.DefaultMixConfig.MinAmount)).
		FloatVar(&config.mixCfg.MinAmount)
	app.Flag("mix-max-amount", "the maximum amount of jobcoins sent").
		Default(str(server.DefaultMixConfig.MaxAmount)).
		FloatVar(&config.mixCfg.MaxAmount)

	app.Flag("pprof-addr", "address for running pprof tools").TCPVar(&config.pprofAddr)

}

func main() {
	if _, err := app.Parse(os.Args[1:]); err != nil {
		app.FatalUsage("command line parsing failed: %v", err)
	}
}

func runServer(_ *kingpin.ParseContext) error {
	mxr, err := server.NewMixer(
		server.WithLogger(l),
		server.WithFee(config.fee),
		server.WithPollConfig(config.pollCfg),
		server.WithMixConfig(config.mixCfg),
	)
	fatalIfError(err, "instantiating mixer failed")

	lis, err := net.Listen("tcp", config.tcpAddr.String())
	fatalIfError(err, "starting TCP listener on %s failed", config.tcpAddr)

	// TODO: log interceptor
	grpcSrv := grpc.NewServer()

	climatic.RegisterMixerServer(grpcSrv, mxr)

	go mxr.Start()
	l.Printf("listening on %s", lis.Addr())
	_ = grpcSrv.Serve(lis)

	return nil
}

func startPprof(_ *kingpin.ParseContext) error {
	if config.pprofAddr == nil {
		return nil
	}

	l.Printf("running pprof server on %s", config.pprofAddr)
	go func() {
		err := http.ListenAndServe(config.pprofAddr.String(), nil)
		fatalIfError(err, "pprof server failed")
	}()

	return nil
}

func fatalIfError(err error, format string, args ...interface{}) {
	if err != nil {
		if format != "" {
			format += ": "
		}
		l.Fatalf(format+"%v", append(args, err)...)
	}
}

func str(val interface{}) string {
	return fmt.Sprintf("%v", val)
}

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/r-medina/climatic"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var config struct {
	register struct {
		mxrTCPAddr *net.TCPAddr
		addrs      []string
	}

	mix struct {
		// jobcoin address
		mxrAddr string
	}
}

var (
	app = kingpin.New("climactl", "climatic client").DefaultEnvars()
)

func init() {
	register := app.Command("register", "register your addresses with a mixer").Action(registerAddrs)
	register.Arg("mixer-ip-addr", "TCP address for mixer service").TCPVar(&config.register.mxrTCPAddr)
	register.Arg("addrs", "deposit addresses for you to receive your jobcoins").
		StringsVar(&config.register.addrs)
}

func main() {
	if _, err := app.Parse(os.Args[1:]); err != nil {
		app.FatalUsage("command line parsing failed: %v", err)
	}
}

func registerAddrs(*kingpin.ParseContext) error {
	mxrTCPAddr := config.register.mxrTCPAddr.String()
	fmt.Printf("dialing %v\n", mxrTCPAddr)
	conn, err := grpc.Dial(
		mxrTCPAddr,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.FailOnNonTempDialError(true),
	)
	app.FatalIfError(err, "dialing %s failed", mxrTCPAddr)
	defer conn.Close()
	client := climatic.NewMixerClient(conn)

	fmt.Printf("registering %v\n", config.register.addrs)
	resp, err := client.Register(
		context.Background(), &climatic.RegisterRequest{Addresses: config.register.addrs},
	)
	app.FatalIfError(err, "registration failed")

	fmt.Println(resp)

	return nil
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/jobcoin"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var config struct {
	register struct {
		mxrTCPAddr *net.TCPAddr
		addrs      []string
	}

	jcClient jobcoin.Client

	send struct {
		fromAddr string
		toAddr   string
		amt      string
	}

	addrInfo struct {
		addr string
	}

	create struct {
		addr string
	}
}

var (
	app = kingpin.New("climactl", "climatic client").DefaultEnvars()
)

func init() {
	register := app.Command("register", "register your addresses with a mixer").Action(registerAddrs)
	register.Arg("mixer-tcp-addr", "TCP address for mixer service").Required().
		TCPVar(&config.register.mxrTCPAddr)
	register.Arg("addrs", "deposit addresses for you to receive your Jobcoins").Required().
		StringsVar(&config.register.addrs)

	send := app.Command("send", "send Jobcoins from an address to an address").
		PreAction(getJobcoinClient).Action(sendJobcoins)
	send.Arg("from-addr", "address from which to send Jobcoins").Required().StringVar(&config.send.fromAddr)
	send.Arg("to-addr", "address to which to send Jobcoins").Required().StringVar(&config.send.toAddr)
	send.Arg("amount", "amount of Jobcoins to send").Required().StringVar(&config.send.amt)

	addrInfo := app.Command("addr-info", "get information about an address").
		PreAction(getJobcoinClient).Action(getAddrInfo)
	addrInfo.Arg("addr", "address for which to lookup information").Required().StringVar(&config.addrInfo.addr)

	create := app.Command("create", "create Jobcoins").PreAction(getJobcoinClient).
		Action(createJobcoins)
	create.Arg("addr", "address to send new Jobcoins").Required().StringVar(&config.create.addr)
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

func sendJobcoins(*kingpin.ParseContext) error {
	fromAddr := config.send.fromAddr
	toAddr := config.send.toAddr
	amt := config.send.amt
	_, err := climatic.ParseFloat(config.send.amt)
	app.FatalIfError(err, "could not parse amount")
	fmt.Printf("sending %s Jobcoins from %v to %v\n", amt, fromAddr, toAddr)
	err = config.jcClient.PostTransaction(fromAddr, toAddr, amt)
	app.FatalIfError(err, "could not send jobcoins")

	return nil
}

func getAddrInfo(*kingpin.ParseContext) error {
	addr := config.addrInfo.addr
	fmt.Printf("getting information about address %v\n", addr)
	addrInfo, err := config.jcClient.GetAddressInfo(addr)
	app.FatalIfError(err, "failed to get address info")
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "\t")
	app.FatalIfError(encoder.Encode(addrInfo), "could not serialize response")

	return nil
}

func createJobcoins(*kingpin.ParseContext) error {
	addr := config.create.addr
	fmt.Printf("creating Jobcoins for address %v\n", addr)
	app.FatalIfError(config.jcClient.Create(addr), "failed to create Jobcoins")

	return nil
}

func getJobcoinClient(*kingpin.ParseContext) error {
	config.jcClient = jobcoin.NewClimaticClient()

	return nil
}

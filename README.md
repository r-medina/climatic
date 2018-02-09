# climatic
Gemini coding challenge.

This repository contains Ricky Medina's solutiion to the Gemini coding challenge
(which will not be explained here). If you want to build the go source files,
this directory should be in your `GOPATH` at `github.com/r-medina/climatic`

This solution is structured in the following way: there is a server, `climasrv`,
and a client `climactl`. The server allows clients to register addresses and
runs the threads that do the mixing.

The basic structure of this repository is as follows:
```
.
├── bin - compiled binaries for common architectures/operating systems to run the code
├── cmd - source code for binaries
│   ├── climactl - client binary
│   └── climasrv - server binary
├── jobcoin - jobcoin API client
│   ├── jctest - mocked client for tests
├── scripts - build/test scripts
└── server - source code for mixer
```

## Use

See the sections below for how to properly configure the server, but there are
pre-compiled binaries that you can use to launch the server and use the
client. If, for example, you're on a relatively recent MacBook, you can run a
sanely-configured server from this directory by doing `./bin/climasrv.darwin-amd64`.

The client binary `./bin/climactl.darwin-amd64` gives you access to all the
jobcoin API endpoints (including create), but also gives you a command
`register` for registering your user addresses with the mixer.

## Design

The mixer functions by having two separate threads:

- one that polls for new transactions that need to be mixed (that is,
  transactions where users sent Jobcoins to a deposit address)
- one that sends out mixed coins.

When the server is instantiated, each of these threads is started.

```go
// Mixer implements the mixer interface and is a Jobcoin mixer.
type Mixer struct {
	// addr is the address at which to collect fees
	addr string

	// jcClient client for interacting with Jobcoin API
	jcClient jobcoin.Client
	// ds is the internal datastore for saving registered user addresses and
	// deposit addresses
	ds Datastore

	// fee is how much fee is charged per deposit.
	// There is a buggy edgecase, however, where if a new deposit happens
	// after a failed attempt to collect a fee, we may only collect a fee on
	// the latter oone.
	fee *big.Float
	// lastSeenTxIdx keeps track of the transaction that the mixer saw.
	// This is useful for the polling loop so it doesn't repeat transactions
	// it has mixed.
	lastSeenTxIdx int

	// outstanding maps deposit addresses to user addresses, amount
	// remaining, and if the fee was paid
	outstanding map[string]*mix
	mtx         sync.Mutex

	// pollCfg configures the polling interval time
	pollCfg PollConfig
	// mixCfg configures the mixing interval times as well as the minimum
	// and maxiumum amounts sent
	mixCfg MixConfig

	log grpclog.Logger
}
```

### Polling

The polling loop finds new deposits to addresses that the mixer is
watching. Every time it runs, the mixer pulls down the entire list of
transactions from the Jobcoin API. Once it has them, the mixer ignores the first
n where n is the number of transactiosn it had already looked at.

After the mixer goes through the new transactions and detects which it has to
mix, it waits a period of time (`MixConfig.InitialDelay`) before adding them to
the datastructure that keeps track of outstanding mixes.

When new mixes are added, they are done so on a per-deposit address basis. THe
mixer keeps track of how much balance is left and if it has collected fees.

### Mixing

The mixing loop picks a deposit address with an outstanding balance and sends a
random amount (this is configurable) to a random user address registered to that
address. The randomness in timing and amounts are all configurable.

When a new mix request is being proccessed, the mixer makes sure a fee is
collected (if needed). If the amount sent is less than the configured fee, The
entire balance is collected.

See the function `mix` in `server/server.go` for detailed comments on the
specifics of how mixing happens.

## Server

The server binary has no commands, but has a lot of available configuration.

```
usage: climasrv [<flags>]

climatic server

Flags:
  --help                    Show context-sensitive help (also try --help-long and --help-man).
  --tcp-addr=               address for TCP listener
  --fee=FEE                 fee to charge people using the service
  --fee-addr=FEE-ADDR       jobcoin address to collect fees
  --poll-delay=10s          mean of delay between polls to jobcoin API
  --poll-dev=3s             the standard deviation of time between polls to jobcoin API
  --poll-min-delay=2s       the minimum delay between polling
  --poll-max-delay=20s      the maximum delay between polling
  --mix-delay=1s            mean of delay between times that jobcoins are mixed
  --mix-dev=250ms           the standard deviation of time between mixes
  --mix-min-delay=50ms      the minimum delay between mixing
  --mix-max-delay=3s        the maximum delay between mixing
  --mix-initial-delay=1m0s  the time between receiving a mix request and first time it is eligible for mixing
  --mix-amount=10           mean of amount of jobcoins sent per transaction
  --mix-dev-amount=8        the standard deviation of jobcoins sent per transaction
  --mix-min-amount=5        the minimum amount of jobcoins sent
  --mix-max-amount=100      the maximum amount of jobcoins sent
  --pprof-addr=PPROF-ADDR   address for running pprof tools
```

A useful example would be:

```bash
./bin/climasrv.darwin-amd64 \
	--tcp-addr :9999 \
	--fee 2.5 \
	--mix-initial-delay 0 \
	--poll-delay 1s \
	--poll-dev 0 \
	--fee-addr fee-addr
```

## Client

The client has several useful commands both for dealing with Joobcoins and mixing them:

```
usage: climactl [<flags>] <command> [<args> ...]

climatic client

Flags:
  --help  Show context-sensitive help (also try --help-long and --help-man).

Commands:
  help [<command>...]
    Show help.

  register <mixer-tcp-addr> <addrs>...
    register your addresses with a mixer

  send <from-addr> <to-addr> <amount>
    send Jobcoins from an address to an address

  addr-info <addr>
    get information about an address

  create <addr>
    create Jobcoins
```

It is important to note that the client makes direct calls the the Jobcoin API
for most of its work. The only time that the client connects to the server is to
register addresses.

In order to use the register command, you have to know where the server is
running. On startup, the server prints out its address. You can configure it at
startup and not worry about it (in the example above I use `:9999`). 

## Jobcoin API client

The API client in the `jobcoin` package includes all the documented endpoints as
well as the `/create` one.

## Scripts

There are three bash scripts included in  `scripts/`.

- `build.sh` builds all the precompiled binaries for running the server and
  client on common architectures
- `ge-pb.sh` compiles the `*.proto` files into Go code
- `test.sh` runs linting on all the go code as well as runs the tests (even
  displays test coverage and tests for race conditionos)

## Caveats

- Testing
- Server float rounding

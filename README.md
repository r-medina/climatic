# climatic
Gemini coding challenge.

This repository contains Ricky Medina's solutiion to the Gemini coding challenge
(which will not be explained here).

This solution is structured in the following way: there is a server, `climasrv`,
and a client `climactl`. 

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


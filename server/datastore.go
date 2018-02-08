package server

import "sync"

// Datastore contains the functions necessary from a datastore for the Mixer
type Datastore interface {
	// Register registers a deposit address with the associated user addresses.
	Register(depositAddr string, usrAddrs []string) error
	// DepositAddresses lists all the deposit addresses.
	DepositAddresses() ([]string, error)
	// UserAddresses lists all the user addresses for a given deposit address.
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

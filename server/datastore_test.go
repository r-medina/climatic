package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemDS(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		depositAddr string
		usrAddrs    []string
	}{
		{depositAddr: "a", usrAddrs: []string{"b"}},
		{depositAddr: "a", usrAddrs: []string{"b, c"}},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			ds := newMemDS()

			err := ds.Register(test.depositAddr, test.usrAddrs)
			require.NoError(err, "failed to register")

			depositAddrs, err := ds.DepositAddresses()
			require.NoError(err, "failed to get deposit addresses")

			require.Equal([]string{test.depositAddr}, depositAddrs, "unexpected deposit addresses")

			usrAddrs, err := ds.UserAddresses(test.depositAddr)
			require.NoError(err, "failed to get user addresses")
			require.Equal(usrAddrs, test.usrAddrs, "unexpected user addresses")
		})
	}

	// tests with existing element
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			ds := newMemDS()
			err := ds.Register("some", []string{"thing"})
			require.NoError(err, "failed to register")

			err = ds.Register(test.depositAddr, test.usrAddrs)
			require.NoError(err, "failed to register")

			depositAddrs, err := ds.DepositAddresses()
			require.NoError(err, "failed to get deposit addresses")

			require.ElementsMatch([]string{test.depositAddr, "some"}, depositAddrs, "unexpected deposit addresses")

			usrAddrs, err := ds.UserAddresses(test.depositAddr)
			require.NoError(err, "failed to get user addresses")
			require.Equal(usrAddrs, test.usrAddrs, "unexpected user addresses")
		})
	}

}

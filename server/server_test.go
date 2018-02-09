package server

import (
	"errors"
	"math/big"
	"testing"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/jobcoin"
	"github.com/r-medina/climatic/jobcoin/jctest"

	"github.com/stretchr/testify/assert"
)

func TestMakeMix(t *testing.T) {
	t.Parallel()

	require := assert.New(t) // this is not working as expected
	parseFloat := makeParseFloat(t)

	tests := []struct {
		desc        string
		outstanding map[string]*mix
		mixReqs     []mixRequest
		want        map[string]*mix
	}{
		{
			desc: "one - no balance or usrAddrs",
			mixReqs: []mixRequest{{
				tx: &jobcoin.Transaction{
					ToAddress: "b",
				},
			}},
			want: map[string]*mix{"b": {}},
		},

		{
			desc: "one with balance and addrs",
			mixReqs: []mixRequest{{
				tx: &jobcoin.Transaction{
					ToAddress: "b",
					Amount:    "2.",
				},
				usrAddrs: []string{"u1", "u2"}},
			},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: parseFloat("2."),
				},
			},
		},

		{
			desc: "two - same deposit addr",
			mixReqs: []mixRequest{
				{
					tx: &jobcoin.Transaction{
						ToAddress: "b",
						Amount:    "2.",
					},
					usrAddrs: []string{"u1", "u2"},
				}, {
					tx: &jobcoin.Transaction{
						ToAddress: "b",
						Amount:    "2.",
					},
					usrAddrs: []string{"u1", "u2"},
				},
			},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: parseFloat("4."),
				},
			},
		},

		{
			desc: "existing - same deposit addr",
			outstanding: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: parseFloat("2."),
				},
			},
			mixReqs: []mixRequest{{
				tx: &jobcoin.Transaction{
					ToAddress: "b",
					Amount:    "2.",
				},
				usrAddrs: []string{"u1", "u2"},
			}},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: parseFloat("4."),
				},
			},
		},

		{
			desc: "existing - new deposit addr",
			outstanding: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: parseFloat("2."),
				},
			},
			mixReqs: []mixRequest{{
				tx: &jobcoin.Transaction{
					ToAddress: "c",
					Amount:    "2.",
				},
				usrAddrs: []string{"u3"},
			}},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: parseFloat("2."),
				},
				"c": {
					usrAddrs:  []string{"u3"},
					remaining: parseFloat("2.00"),
				},
			},
		},
	}

	mxr := &Mixer{ds: newMemDS()}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			mxr.outstanding = test.outstanding
			if test.outstanding == nil {
				mxr.outstanding = map[string]*mix{}
			}

			mxr.makeMix(test.mixReqs)

			require.Equal(test.want, mxr.outstanding, "ending state equal")
		})
	}
}

func TestUpdateRemaining(t *testing.T) {
	t.Parallel()

	require := assert.New(t) // this is not working as expected
	parseFloat := makeParseFloat(t)

	tests := []struct {
		remaining *big.Float
		err       error
		del       bool
	}{
		{remaining: parseFloat("1"), del: false},
		{remaining: parseFloat("0"), del: true},
		{err: errors.New("some error"), del: false},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			jcClient := &jctest.MockClient{}
			mxr, err := NewMixer(WithJobcoinClient(jcClient))
			require.NoError(err)

			jcClient.AddrInfo = func() (*jobcoin.AddressInfo, error) {
				var addrInfo *jobcoin.AddressInfo
				if test.remaining != nil {
					addrInfo = &jobcoin.AddressInfo{
						Balance: test.remaining.String(),
					}
				}
				return addrInfo, test.err
			}

			m := &mix{}
			del, err := mxr.updateRemaining(m, "")
			require.Equal(test.err, err)
			require.Equal(test.del, del)
			if test.err != nil {
				require.Equal(test.remaining, m.remaining)
			}
		})
	}

}

func TestCollectFee(t *testing.T) {
	t.Parallel()

	require := assert.New(t) // this is not working as expected
	parseFloat := makeParseFloat(t)

	tests := []struct {
		fee  *big.Float
		m    *mix
		err  error
		want *mix
	}{
		{
			fee:  parseFloat("3"),
			m:    &mix{remaining: parseFloat("10")},
			want: &mix{remaining: parseFloat("7."), feePaid: true},
		},

		{
			fee:  parseFloat("3"),
			m:    &mix{remaining: parseFloat("2")},
			want: &mix{remaining: big.NewFloat(0), feePaid: true},
		},

		{
			fee:  parseFloat("0"),
			m:    &mix{remaining: parseFloat("2")},
			want: &mix{remaining: parseFloat("2")},
		},

		{
			fee:  parseFloat("100"),
			m:    &mix{remaining: parseFloat("0")},
			want: &mix{remaining: parseFloat("0"), feePaid: false},
		},

		{
			fee:  parseFloat("3"),
			m:    &mix{remaining: parseFloat("10")},
			err:  errors.New("err"),
			want: &mix{remaining: parseFloat("10"), feePaid: false},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			jcClient := &jctest.MockClient{}
			mxr, err := NewMixer(
				WithFee(test.fee),
				WithJobcoinClient(jcClient),
			)
			require.NoError(err)

			if err := test.err; err != nil {
				jcClient.Post = func() error {
					return err
				}
			}

			err = mxr.collectFee(test.m, "addr")
			if test.err != nil {
				require.Equal(test.err, err)
			}

			// have to test it a little more hands on bc big.Float
			// doesn't play great with equality checking

			want, _ := test.want.remaining.Float64()
			got, _ := test.m.remaining.Float64()
			require.InDelta(want, got, 0)
			require.Equal(test.want.feePaid, test.m.feePaid)
		})
	}
}

// needed to get right precision
func makeParseFloat(t *testing.T) func(v string) *big.Float {
	t.Helper()

	require := assert.New(t) // this is not working as expected

	return func(v string) *big.Float {
		t.Helper()

		f, err := climatic.ParseFloat(v)
		require.NoError(err, "failed parsing float")
		return f
	}
}

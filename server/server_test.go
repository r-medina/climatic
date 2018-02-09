package server

import (
	"math/big"
	"testing"

	"github.com/r-medina/climatic"
	"github.com/r-medina/climatic/jobcoin"

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
					remaining: parseFloat("2."),
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

package server

import (
	"testing"

	"github.com/r-medina/climatic/jobcoin"

	"github.com/stretchr/testify/require"
)

func TestMakeMiix(t *testing.T) {
	require := require.New(t) // this is not working as expected

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
					Amount:    2.,
				},
				usrAddrs: []string{"u1", "u2"}},
			},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: 2.,
				},
			},
		},

		{
			desc: "two - same deposit addr",
			mixReqs: []mixRequest{
				{
					tx: &jobcoin.Transaction{
						ToAddress: "b",
						Amount:    2.,
					},
					usrAddrs: []string{"u1", "u2"},
				}, {
					tx: &jobcoin.Transaction{
						ToAddress: "b",
						Amount:    2.,
					},
					usrAddrs: []string{"u1", "u2"},
				},
			},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: 4.,
				},
			},
		},

		{
			desc: "existing - same deposit addr",
			outstanding: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: 2.,
				},
			},
			mixReqs: []mixRequest{{
				tx: &jobcoin.Transaction{
					ToAddress: "b",
					Amount:    2.,
				},
				usrAddrs: []string{"u1", "u2"},
			}},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: 4.,
				},
			},
		},

		{
			desc: "existing - new deposit addr",
			outstanding: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: 2.,
				},
			},
			mixReqs: []mixRequest{{
				tx: &jobcoin.Transaction{
					ToAddress: "c",
					Amount:    2.,
				},
				usrAddrs: []string{"u3"},
			}},
			want: map[string]*mix{
				"b": {
					usrAddrs:  []string{"u1", "u2"},
					remaining: 2.,
				},
				"c": {
					usrAddrs:  []string{"u3"},
					remaining: 3.,
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

			require.Equal(mxr.outstanding, test.want, "ending state equal")
		})
	}
}

package jobcoin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetAddressInfo(t *testing.T) {
	t.Parallel()

	require := assert.New(t) // this is not working as expected

	tests := []struct {
		desc     string
		addrInfo AddressInfo
	}{
		{desc: "empty"},
		{desc: "balance, no txs", addrInfo: AddressInfo{
			Balance: "2.0",
		}},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					body, err := json.Marshal(test.addrInfo)
					if err != nil {
						t.Fatalf("unexpected error marshalling body: %v", err)
					}

					_, _ = rw.Write(body)

				}),
			)
			testClient := NewClimaticClient(WithAPIAddress(server.URL))

			addrInfo, err := testClient.GetAddressInfo("")
			require.NoError(err, "failed GetAddressInfo")
			require.Equal(addrInfo, &test.addrInfo, "unexpected output")
		})
	}
}

func TestGetTransactions(t *testing.T) {
	t.Parallel()

	require := assert.New(t) // this is not working as expected

	makeTime := func(timeStr string) time.Time {
		tm, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			t.Fatalf("unexpected error parsing time: %v", err)
		}

		return tm
	}

	tests := []struct {
		desc string
		txs  []*Transaction
	}{
		{desc: "1 tx", txs: []*Transaction{{
			Timestamp: makeTime("2014-04-22T13:10:01.210Z"),
		}}},

		{desc: "2 txs", txs: []*Transaction{{
			Timestamp: makeTime("2014-04-22T13:10:01.210Z"),
			ToAddress: "BobsAddress",
			Amount:    "50.35",
		}, {
			Timestamp:   makeTime("2014-04-23T18:25:43.511Z"),
			FromAddress: "BobsAddress",
			ToAddress:   "AlicesAddress",
			Amount:      "30.1",
		}}},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					body, err := json.Marshal(test.txs)
					require.NoError(err, "failed to marshal body")
					_, _ = rw.Write(body)

				}),
			)
			testClient := NewClimaticClient(WithAPIAddress(server.URL))

			txs, err := testClient.GetTransactions()
			require.NoError(err, "failed calling GetTransactions")
			require.Equal(txs, test.txs, "expected same output")

		})
	}
}

func TestPostTransaction(t *testing.T) {
	t.Parallel()

	require := assert.New(t) // this is not working as expected

	tests := []struct {
		desc   string
		errStr string
	}{
		{desc: "success"},
		{desc: "some error", errStr: "some err"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					var body []byte
					if test.errStr == "" {
						body = []byte(`{"status":"OK"}`)
					} else {
						body = []byte(`{"error":"` + test.errStr + `"}`)
						rw.WriteHeader(422)
					}

					_, _ = rw.Write(body)
				}),
			)
			testClient := NewClimaticClient(WithAPIAddress(server.URL))

			err := testClient.PostTransaction("a", "b", "1.")
			if err != nil {
				require.Equal(test.errStr, err.Error(), "unexpected error")
			}
		})
	}
}

package jobcoin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestGetAddressInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		addrInfo AddressInfo
	}{
		{desc: "empty"},
		{desc: "balance, no txs", addrInfo: AddressInfo{
			Balance: 2.0,
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
			if err != nil {
				t.Fatalf("unexpected error getting address info: %v", err)
			}

			if want, got := &test.addrInfo, addrInfo; !reflect.DeepEqual(got, want) {
				t.Fatalf("expected %v, got %v", want, got)
			}
		})
	}
}

func TestGetTransactions(t *testing.T) {
	t.Parallel()

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
			Amount:    50.35,
		}, {
			Timestamp:   makeTime("2014-04-23T18:25:43.511Z"),
			FromAddress: "BobsAddress",
			ToAddress:   "AlicesAddress",
			Amount:      30.1,
		}}},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					body, err := json.Marshal(test.txs)
					if err != nil {
						t.Fatalf("unexpected error marshalling body: %v", err)
					}

					_, _ = rw.Write(body)

				}),
			)
			testClient := NewClimaticClient(WithAPIAddress(server.URL))

			txs, err := testClient.GetTransactions()
			if err != nil {
				t.Fatalf("unexpected error getting transactions: %v", err)
			}

			if want, got := test.txs, txs; !reflect.DeepEqual(got, want) {
				t.Fatalf("expected %+v, got %+v", want[0].Timestamp, got[0].Timestamp)
			}
		})
	}
}

func TestPostTransaction(t *testing.T) {
	t.Parallel()

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
					var val interface{}
					if test.errStr == "" {
						val = map[string]string{"status": "OK"}
					} else {
						val = struct{ error string }{error: test.errStr}
					}

					body, err := json.Marshal(val)
					if err != nil {
						t.Fatalf("unexpected error marshalling body: %v", err)
					}

					if test.errStr == "" {
						rw.WriteHeader(422)
					}

					_, _ = rw.Write(body)
				}),
			)
			testClient := NewClimaticClient(WithAPIAddress(server.URL))

			err := testClient.PostTransaction("a", "b", 1.)
			if err != nil {
				if want, got := test.errStr, err.Error(); !reflect.DeepEqual(got, want) {
					t.Fatalf("expected %v, got %v", want, got)
				}
			}
		})
	}
}

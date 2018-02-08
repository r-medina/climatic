// TODO: use pkg/errors and make better errors.

package jobcoin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Client represents a Jobcoin API client.
type Client interface {
	// GetAddressInfo returns all the transactions and the balance for an address.
	GetAddressInfo(addr string) (*AddressInfo, error)
	// GetTransactions returns all the transactions in the jobcoin history.
	GetTransactions() ([]*Transaction, error)
	// PostTransaction sends jobcoin.
	PostTransaction(fromAddr, toAddr string, amt float64) error
	// Create creates 50 Jobcons out of thin air.
	Create(addr string) error
}

// AddressInfo contains all the data relating to a Jobcoin address.
type AddressInfo struct {
	Balance      float64        `json:"balance,string"`
	Transactions []*Transaction `json:"transactions"`
}

// Transaction contains information about a transaction.
type Transaction struct {
	Timestamp   time.Time `json:"time,string"`
	FromAddress string    `json:"fromAddress"`
	ToAddress   string    `json:"toAddress"`
	Amount      float64   `json:"amount,string"`
}

// ClimaticClient uses the https://jobcoin.gemini.com/climatic/api.
type ClimaticClient struct {
	apiAddr    string
	httpClient *http.Client
}

var _ Client = (*ClimaticClient)(nil)

// NewClimaticClient returns a Client implementation that uses the climatic API.
func NewClimaticClient(opts ...Option) *ClimaticClient {
	cli := &ClimaticClient{
		apiAddr:    "https://jobcoin.gemini.com/climatic",
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(cli)
	}

	return cli
}

// Option customizes a ClimaticClient
type Option func(*ClimaticClient)

// WithHTTPClient allows the specification of an HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(cli *ClimaticClient) {
		cli.httpClient = httpClient
	}
}

// WithAPIAddress allows for the specification of a custom API address.
// This is useful for testing.
func WithAPIAddress(apiAddr string) Option {
	return func(cli *ClimaticClient) {
		cli.apiAddr = apiAddr
	}
}

// GetAddressInfo returns all the transactions and the balance for an address.
func (cli *ClimaticClient) GetAddressInfo(addr string) (*AddressInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/addresses/%s", cli.apiAddr, addr), nil)
	if err != nil {
		return nil, err
	}

	res, err := cli.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTPClient.DO failed")
	}
	defer res.Body.Close()

	addrInfo := &AddressInfo{}
	decoder := json.NewDecoder(res.Body)
	if err = decoder.Decode(addrInfo); err != nil {
		return nil, errors.Wrap(err, "Decode failed")
	}

	return addrInfo, nil
}

// GetTransactions returns all the transactions in the jobcoin history.
func (cli *ClimaticClient) GetTransactions() ([]*Transaction, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/transactions", cli.apiAddr), nil)
	if err != nil {
		return nil, err
	}

	res, err := cli.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	txs := []*Transaction{}
	decoder := json.NewDecoder(res.Body)
	if err = decoder.Decode(&txs); err != nil {
		return nil, err
	}

	return txs, nil
}

// PostTransaction sends jobcoin.
func (cli *ClimaticClient) PostTransaction(fromAddr, toAdddr string, amt float64) error {
	body := &bytes.Buffer{}
	encoder := json.NewEncoder(body)
	err := encoder.Encode(struct {
		FromAddress string  `json:"fromAddress"`
		ToAddress   string  `json:"toAddress"`
		Amount      float64 `json:"amount,string"`
	}{
		FromAddress: fromAddr,
		ToAddress:   toAdddr,
		Amount:      amt,
	})
	if err != nil {
		return err
	}

	res, err := cli.httpClient.Post(
		fmt.Sprintf("%s/api/transactions", cli.apiAddr), "application/json", body,
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		apiErr := map[string]interface{}{}
		decoder := json.NewDecoder(res.Body)
		if err = decoder.Decode(&apiErr); err != nil {
			return errors.Errorf("API error: %d", res.StatusCode)
		}
		buf, err := json.Marshal(apiErr["error"])
		if err != nil {
			return errors.Errorf("API error: %d", res.StatusCode)
		}

		return errors.New(string(buf))
	}

	return nil
}

// Create creates Jobcoins for the given address.
func (cli *ClimaticClient) Create(addr string) error {
	body := strings.NewReader("address=" + addr)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/create", cli.apiAddr), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := cli.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return errors.Errorf("API error: %d", res.StatusCode)
	}

	return nil
}

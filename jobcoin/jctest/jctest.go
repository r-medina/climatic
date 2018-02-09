package jctest

import "github.com/r-medina/climatic/jobcoin"

// MockClient mocks a jobcoin client.
type MockClient struct {
	AddrInfo     func() (*jobcoin.AddressInfo, error)
	Transactions func() ([]*jobcoin.Transaction, error)
	Post         func() error
}

var _ jobcoin.Client = (*MockClient)(nil)

// GetAddressInfo calls func in mock.
func (cli *MockClient) GetAddressInfo(string) (*jobcoin.AddressInfo, error) {
	if cli.AddrInfo == nil {
		return nil, nil
	}
	return cli.AddrInfo()
}

// GetTransactions calls func in mock.
func (cli *MockClient) GetTransactions() ([]*jobcoin.Transaction, error) {
	if cli.Transactions == nil {
		return nil, nil
	}
	return cli.Transactions()
}

// PostTransaction calls func in mock.
func (cli *MockClient) PostTransaction(_, _, _ string) error {
	if cli.Post == nil {
		return nil
	}
	return cli.Post()
}

// Create calls func in mock.
func (cli *MockClient) Create(string) error {
	return nil
}

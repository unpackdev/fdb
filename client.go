package fdb

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c Client) Read(address string) error {
	return nil
}

func (c Client) Write(address string) error {
	return nil
}

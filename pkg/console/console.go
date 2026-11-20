package console

import (
	"github.com/convox/convox/sdk"
)

type Client struct {
	*sdk.Client
	handler Handler
	machine string
}

type Handler interface {
	SettingReadKey(string, string) (string, error)
	SettingWriteKey(string, string, string) error
	Writef(string, ...interface{}) error
}

func NewClient(endpoint string, rack string, handler Handler) (*Client, error) {
	s, err := sdk.New(endpoint)
	if err != nil {
		return nil, err
	}

	cl := &Client{
		Client:  s,
		handler: handler,
	}

	s.Authenticator = cl.authenticator
	s.Rack = rack
	s.Session = cl.session

	return cl, nil
}

func (c *Client) SetMachine(id, name string) {
	c.Client.MachineID = id
	c.machine = name
}

// CloudMachine returns the machine name as the user gave it, or "" when not on a machine.
func (c *Client) CloudMachine() string {
	return c.machine
}

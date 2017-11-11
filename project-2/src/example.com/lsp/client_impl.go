// Contains the implementation of a LSP client.

package lsp

import (
	"math/rand"
	"net"
)

type client struct {
	id      int
	conn    net.Conn
	timeout int
}

// NewClient creates, initiates, and returns a new client. This function
// should return after a connection with the server has been established
// (i.e., the client has received an Ack message from the server in response
// to its connection request), and should return a non-nil error if a
// connection could not be made (i.e., if after K epochs, the client still
// hasn't received an Ack message from the server in response to its K
// connection requests).
//
// hostport is a colon-separated string identifying the server's host address
// and port number (i.e., "localhost:9999").
func NewClient(hostport string, params *Params) (Client, error) {
	conn, err := net.Dial("udp", hostport)
	if err != nil {
		return nil, err
	}
	return &client{
		id:      rand.Int() + 1,
		conn:    conn,
		timeout: params.EpochLimit,
	}, nil
}

func (c *client) ConnID() int {
	return c.id
}

func (c *client) Read() ([]byte, error) {
	buffer := make([]byte, 1024)
	_, err := c.conn.Read(buffer)
	return buffer, err
}

func (c *client) Write(payload []byte) error {
	_, err := c.conn.Write(payload)
	return err
}

func (c *client) Close() error {
	return c.conn.Close()
}

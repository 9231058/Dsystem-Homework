// Contains the implementation of a LSP client.

package lsp

import (
	"encoding/json"

	net "../lspnet"
)

type client struct {
	id      int
	udpConn *net.UDPConn
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
	addr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	// Send connection request
	mt := NewConnect()
	b, _ := json.Marshal(mt)
	_, err = conn.Write(b)
	if err != nil {
		return nil, err
	}

	// Receive connection acceptance
	buff := make([]byte, 1024)
	var mr Message
	nbytes, err := conn.Read(buff)
	if err != nil {
		return nil, err
	}
	buff = buff[:nbytes]
	err = json.Unmarshal(buff, &mt)
	if err != nil {
		return nil, err
	}

	return &client{
		id:      mr.ConnID,
		udpConn: conn,
		timeout: params.EpochLimit,
	}, nil
}

func (c *client) ConnID() int {
	return c.id
}

func (c *client) Read() ([]byte, error) {
	buffer := make([]byte, 1024)
	_, err := c.udpConn.Read(buffer)
	return buffer, err
}

func (c *client) Write(payload []byte) error {
	_, err := c.udpConn.Write(payload)
	return err
}

func (c *client) Close() error {
	return c.udpConn.Close()
}

// Contains the implementation of a LSP client.

package lsp

import (
	"encoding/json"
	"log"
	"time"

	net "../lspnet"
)

type client struct {
	id int
	sq int

	timeout int
	retries int
	window  chan Message

	udpConn *net.UDPConn
	acks    chan Message
	data    chan Message
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

	// Receive connection acceptance or retry
	epochEvents := time.Tick(time.Duration(params.EpochMillis) * time.Millisecond)
	messageEvents := make(chan Message)

	go func() {
		buff := make([]byte, 1024)
		var mr Message
		nbytes, err := conn.Read(buff)
		if err != nil {
			log.Fatal(err)
		}
		buff = buff[:nbytes]
		err = json.Unmarshal(buff, &mr)
		if err != nil {
			log.Fatal(err)
		}
		messageEvents <- mr
	}()

	for {
		select {
		case <-epochEvents:
			// Timeout
			mt := NewConnect()
			b, _ := json.Marshal(mt)
			_, err = conn.Write(b)
			if err != nil {
				return nil, err
			}

		case mr := <-messageEvents:
			// Connection is accpeted
			cli := &client{
				id: mr.ConnID,
				sq: 1,

				udpConn: conn,
				acks:    make(chan Message, params.WindowSize),
				data:    make(chan Message, params.WindowSize),

				timeout: params.EpochMillis,
				retries: params.EpochLimit,
				window:  make(chan Message, params.WindowSize-1),
			}
			go cli.receiver()
			return cli, nil

		}
	}
}

func (c *client) ConnID() int {
	return c.id
}

func (c *client) receiver() {
	var expectedSeqNum = 1
	for {
		// Read incomming message
		buff := make([]byte, 1024)
		var m Message
		nbytes, err := c.udpConn.Read(buff)
		if err != nil {
			log.Fatal(err)
			continue
		}
		buff = buff[:nbytes]
		err = json.Unmarshal(buff, &m)
		if err != nil {
			log.Fatal(err)
			continue
		}

		switch m.Type {
		case MsgAck:
			c.acks <- m
		case MsgData:
			if m.SeqNum == expectedSeqNum {
				expectedSeqNum++
				c.data <- m
			} else {
			}

			// Send ACK
			mt := NewAck(c.id, expectedSeqNum)
			b, _ := json.Marshal(mt)
			c.udpConn.Write(b)
		}
	}
}

func (c *client) sender() {
	var lastMessage *Message
	for {
		if lastMessage != nil {
			select {}
		} else {
		}
	}
}

func (c *client) Read() ([]byte, error) {
	m := <-c.data
	return m.Payload, nil
}

func (c *client) Write(payload []byte) error {
	// Create message
	m := NewData(c.id, c.sq, len(payload), payload)
	c.sq++

	// Marshaling and send
	b, _ := json.Marshal(m)
	_, err := c.udpConn.Write(b)
	return err
}

func (c *client) Close() error {
	return c.udpConn.Close()
}

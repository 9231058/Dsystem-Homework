// Contains the implementation of a LSP client.

package lsp

import (
	"errors"
	"fmt"
	"log"
	"time"

	net "../lspnet"
)

type client struct {
	id      int
	udpConn *net.UDPConn

	incoming chan *Message

	tbuffer map[int]*Message
	tmsg    chan *Message
	tsq     int

	rbuffer map[int][]byte
	rmsg    chan []byte
	rsq     int

	err    error
	status Status
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
		log.Fatal(err)
	}

	// get connection
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatal(err)
	}

	cli := &client{
		id:      -1,
		udpConn: conn,

		incoming: make(chan *Message, 1024),

		tbuffer: make(map[int]*Message),
		tmsg:    make(chan *Message, 1000),
		tsq:     0,

		rbuffer: make(map[int][]byte),
		rmsg:    make(chan []byte, 1000),
		rsq:     1,

		err:    nil,
		status: NOT_CLOSING,
	}

	statusSignal := make(chan int)

	// send connect message
	// new connect message
	connectMessage := NewConnect()
	cli.tbuffer[cli.tsq] = connectMessage
	cli.tsq++
	WriteMessage(conn, nil, connectMessage)

	go cli.receiver()
	go cli.eventLoopForClient(statusSignal, params)

	status := <-statusSignal

	if status == 0 {
		return cli, nil
	}

	return cli, errors.New("client creation failed")
}

func (c *client) ConnID() int {
	return c.id
}

func (c *client) Read() ([]byte, error) {
	for {
		select {
		case data := <-c.rmsg:
			return data, nil
		default:
			if c.err != nil {
				return nil, c.err
			}
		}
		time.Sleep(time.Microsecond)
	}
}

func (c *client) Write(payload []byte) error {
	message := NewData(c.id, -1, len(payload), payload)
	c.tmsg <- message

	return nil
}

func (c *client) Close() error {
	if c.status == NOT_CLOSING {
		c.status = START_CLOSING
	}

	for {
		if c.status == HANDLER_CLOSED {
			c.udpConn.Close()
		}
		if c.status == CONNECTION_CLOSED {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

func (c *client) receiver() {
	for {
		m, _, err := ReadMessage(c.udpConn)
		if err != nil {
			if c.id > 0 {
				c.status = CONNECTION_CLOSED
				return
			}
		} else {
			c.incoming <- m
		}
	}
}

func (c *client) eventLoopForClient(statusSignal chan int, params *Params) {
	var epochCount int
	timer := time.NewTimer(time.Duration(params.EpochMillis) * time.Millisecond)

	for {
		if c.status == START_CLOSING && len(c.tbuffer) == 0 && len(c.rbuffer) == 0 && len(c.tmsg) == 0 {
			c.status = HANDLER_CLOSED
			return
		}

		if c.err != nil {
			c.status = HANDLER_CLOSED
			return
		}

		minUnAcked := c.tsq
		for sq := range c.tbuffer {
			if minUnAcked > sq {
				minUnAcked = sq
			}
		}

		select {
		case m := <-c.incoming:
			epochCount = 0

			switch m.Type {
			case MsgData:
				// why we need this ? :thinking:
				if m.Size > len(m.Payload) {
					continue
				}
				m.Payload = m.Payload[0:m.Size]

				// save data into buffer
				_, exists := c.rbuffer[m.SeqNum]
				if !exists {
					c.rbuffer[m.SeqNum] = m.Payload
				}

				if m.SeqNum == c.rsq {
					i := c.rsq
					for {
						data, exists := c.rbuffer[i]
						if !exists {
							break
						}
						c.rmsg <- data
						c.rsq++
						delete(c.rbuffer, i)
						i++
					}
				}

				// send ack
				response := NewAck(c.id, m.SeqNum)
				go WriteMessage(c.udpConn, nil, response)

			case MsgAck:
				_, exists := c.tbuffer[m.SeqNum]
				if exists {
					if m.Type == MsgConnect && c.id < 0 {
						c.id = m.ConnID
						close(statusSignal)
					}
					delete(c.tbuffer, m.SeqNum)
				}
			}

		case <-timer.C:
			epochCount++

			if epochCount == params.EpochLimit {
				c.err = fmt.Errorf("client %d: Connection Lost", c.id)
			} else {
				if c.id < 0 {
					m := NewConnect()
					go WriteMessage(c.udpConn, nil, m)
				} else {
					if c.rsq == 1 && len(c.rbuffer) == 0 {
						m := NewAck(c.id, 0)
						go WriteMessage(c.udpConn, nil, m)
					}
				}
				for _, m := range c.tbuffer {
					go WriteMessage(c.udpConn, nil, m)
				}
			}

			timer.Reset(time.Duration(params.EpochMillis) * time.Millisecond)

		default:
			time.Sleep(time.Nanosecond)

			if c.tsq-minUnAcked < params.WindowSize {
				select {
				case m := <-c.tmsg:
					m.SeqNum = c.tsq

					c.tbuffer[c.tsq] = m
					c.tsq++
					go WriteMessage(c.udpConn, nil, m)
				default:
				}
			}
		}
	}
}

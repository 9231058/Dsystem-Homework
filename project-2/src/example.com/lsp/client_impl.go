// Contains the implementation of a LSP client.

package lsp

import (
	"encoding/json"
	"log"
	"time"

	net "../lspnet"
)

type client struct {
	id  int
	tsq int // Transmitted sq
	rsq int // Received sq

	retries  int
	windows  int
	timer    <-chan time.Time
	incoming chan Message

	udpConn *net.UDPConn
	rbuffer map[int]Message
	rmsg    chan Message

	tbuffer map[int]Message
	tmsg    chan Message
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
			// TODO: epoch.limits

		case mr := <-messageEvents:
			// Connection is accpeted
			cli := &client{
				id:  mr.ConnID,
				tsq: 1,
				rsq: 1,

				udpConn: conn,
				tbuffer: make(map[int]Message),
				tmsg:    make(chan Message, 1024),

				rbuffer: make(map[int]Message),
				rmsg:    make(chan Message, 1024),

				timer:   time.Tick(time.Duration(params.EpochMillis) * time.Millisecond),
				retries: params.EpochLimit,
				windows: params.WindowSize,

				incoming: make(chan Message, 1024),
			}
			go cli.receiver()
			go cli.handle()
			return cli, nil

		}
	}
}

func (c *client) ConnID() int {
	return c.id
}

func (c *client) receiver() {
	for {
		// Read incomming message
		buff := make([]byte, 1024)
		var m Message
		nbytes, err := c.udpConn.Read(buff)
		if err != nil {
			log.Fatal(err)
			return
		}
		buff = buff[:nbytes]
		err = json.Unmarshal(buff, &m)
		if err != nil {
			log.Fatal(err)
			return
		}

		c.incoming <- m
	}
}

func (c *client) handle() {
	var minUnacked int
	var maxUnacked int

	for {
		minUnacked = maxUnacked
		for i := range c.tbuffer {
			if minUnacked > i {
				minUnacked = i
			}
		}

		select {
		case m := <-c.incoming:
			switch m.Type {
			case MsgAck:
				if _, ok := c.tbuffer[m.SeqNum]; ok == true {
					delete(c.tbuffer, m.SeqNum)
				}
			case MsgData:
				if m.SeqNum == c.rsq {
					// in order
					c.rmsg <- m
					c.rsq++

					for {
						m, ok := c.rbuffer[c.rsq]
						if !ok {
							break
						}
						c.rsq++
						c.rmsg <- m
						delete(c.rbuffer, c.rsq)
					}
				} else {
					// out of order
					c.rbuffer[m.SeqNum] = m
				}

				// Send ACK
				a := NewAck(c.id, c.rsq-1)

				go func() {
					b, _ := json.Marshal(a)
					c.udpConn.Write(b)
				}()
			}
		case <-c.timer:
		default:
			if maxUnacked-minUnacked < c.windows {
				select {
				case m := <-c.tmsg:
					c.tbuffer[m.SeqNum] = m

					go func() {
						// Marshaling and send
						b, _ := json.Marshal(m)
						c.udpConn.Write(b)
					}()
				default:
				}
			}
		}
	}
}

func (c *client) Read() ([]byte, error) {
	m := <-c.rmsg

	return m.Payload, nil
}

func (c *client) Write(payload []byte) error {
	// Create message
	m := NewData(c.id, c.tsq, len(payload), payload)
	c.tsq++

	c.tmsg <- *m

	return nil
}

func (c *client) Close() error {
	return c.udpConn.Close()
}

// Contains the implementation of a LSP server.

package lsp

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	net "../lspnet"
)

type server struct {
	udpConn *net.UDPConn

	lastConnID int

	incoming chan Message
	outgoing chan Message

	rmsg chan Message

	connections *sync.Map
}

type conn struct {
	id  int
	rsq int
	tsq int

	timer   <-chan time.Time
	retries int
	windows int

	addr    *net.UDPAddr
	udpConn *net.UDPConn

	tmsg    chan Message
	tbuffer map[int]Message

	rmsg    chan Message
	rbuffer map[int]Message

	incoming chan Message
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	uc, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	s := &server{
		udpConn:    uc,
		lastConnID: 73,

		incoming: make(chan Message, 1024),
		outgoing: make(chan Message, 1024),

		rmsg: make(chan Message, 1024),

		connections: new(sync.Map),
	}
	// Handles incomming messages
	go s.handle()
	go s.receiver(params)
	return s, nil
}

func (s *server) receiver(params *Params) {
	for {
		// Read incomming message
		buff := make([]byte, 2000)
		var m Message
		nbytes, addr, err := s.udpConn.ReadFromUDP(buff)
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

		// Connection setup
		if m.Type == MsgConnect {
			connID := s.lastConnID
			s.lastConnID++

			c := &conn{
				id:  connID,
				tsq: 1,
				rsq: 1,

				addr:    addr,
				udpConn: s.udpConn,

				timer:   time.Tick(time.Duration(params.EpochMillis) * time.Millisecond),
				retries: params.EpochLimit,
				windows: params.WindowSize,

				tmsg:    make(chan Message, 1024),
				tbuffer: make(map[int]Message),

				rmsg:    s.rmsg,
				rbuffer: make(map[int]Message),

				incoming: make(chan Message, 1024),
			}
			s.connections.Store(connID, c)
			go c.conn()

			a := NewAck(connID, 0)
			go func() {
				b, _ := json.Marshal(a)
				s.udpConn.WriteToUDP(b, addr)
			}()
		} else {
			s.incoming <- m
		}
	}

}

func (s *server) handle() {
	for {
		m := <-s.incoming
		if v, ok := s.connections.Load(m.ConnID); ok == true {
			c := v.(*conn)
			c.incoming <- m
		}
	}
}

func (c *conn) conn() {
	var minUnacked int
	var maxUnacked int
	var epochCount int

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
				a := NewAck(c.id, m.SeqNum)

				go func() {
					b, _ := json.Marshal(a)
					c.udpConn.WriteToUDP(b, c.addr)
				}()
			}
		case <-c.timer:
			epochCount++

			if epochCount == c.retries {
				// TODO: connection lost
			} else {
				if c.rsq == 1 {
					m := NewAck(c.id, 0)
					go func() {
						// Marshaling and send
						b, _ := json.Marshal(m)
						c.udpConn.WriteToUDP(b, c.addr)
					}()
				}
				for _, m := range c.tbuffer {
					go func() {
						// Marshaling and send
						b, _ := json.Marshal(m)
						c.udpConn.WriteToUDP(b, c.addr)

					}()
				}
			}
		default:
			if maxUnacked-minUnacked < c.windows {
				select {
				case m := <-c.tmsg:
					c.tbuffer[m.SeqNum] = m

					go func() {
						// Marshaling and send
						b, _ := json.Marshal(m)
						c.udpConn.WriteToUDP(b, c.addr)
					}()
				default:
				}
			}
		}
	}

}

func (s *server) Read() (int, []byte, error) {
	message := <-s.rmsg

	return message.ConnID, message.Payload, nil
}

func (s *server) Write(connID int, payload []byte) error {
	if v, ok := s.connections.Load(connID); ok == true {
		c := v.(*conn)

		m := NewData(connID, c.tsq, len(payload), payload)
		c.tsq++
		c.tmsg <- *m
	}

	return nil
}

func (s *server) CloseConn(connID int) error {
	return nil
}

func (s *server) Close() error {
	return s.udpConn.Close()
}

// Contains the implementation of a LSP server.

package lsp

import (
	"fmt"
	"strconv"
	"time"

	net "../lspnet"
)

type server struct {
	clients    map[int]*clientInfo
	lastConnID int

	udpConn *net.UDPConn

	incoming chan *addressableMessage
	outgoing chan *Message

	rmsg chan *clientData

	status *status
	err    chan *clientError
	cls    chan int
	bcls   chan int
}

type clientInfo struct {
	id      int
	addr    *net.UDPAddr
	udpConn *net.UDPConn

	incoming chan *Message

	tbuffer map[int]*Message
	tmsg    chan *Message
	tsq     int

	rbuffer map[int][]byte
	rsq     int

	timer   <-chan time.Time
	retries int
	windows int

	status *status
}

type clientData struct {
	id   int
	data []byte
}

type clientError struct {
	id  int
	err error
}

type addressableMessage struct {
	Message
	addr *net.UDPAddr
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	s := &server{
		clients:    make(map[int]*clientInfo),
		lastConnID: 73,

		udpConn: conn,

		incoming: make(chan *addressableMessage, 10000),
		outgoing: make(chan *Message, 10000),

		rmsg: make(chan *clientData, 10000),

		status: newStatus(),
		err:    make(chan *clientError, 1024),
		cls:    make(chan int, 1024),
		bcls:   make(chan int),
	}

	go s.receiver()
	go s.handle(params)

	return s, nil
}

func (s *server) Read() (int, []byte, error) {
	select {
	case element := <-s.rmsg:
		return element.id, element.data, nil
	case lost := <-s.err:
		return lost.id, nil, lost.err
	}
}

func (s *server) Write(connID int, payload []byte) error {
	message := NewData(connID, -1, len(payload), payload)
	s.outgoing <- message

	return nil
}

func (s *server) CloseConn(connID int) error {
	s.cls <- connID

	return nil
}

func (s *server) Close() error {
	s.status.set(startClosing)

	close(s.bcls)

	for {
		if s.status.get() == handlerClosed {
			s.udpConn.Close()
		}
		if s.status.get() == connectionClosed {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *server) receiver() {
	for {
		m, addr, err := ReadMessage(s.udpConn)
		if err != nil {
			if s.status.get() == handlerClosed {
				s.status.set(connectionClosed)
				return
			}
		} else {
			s.incoming <- &addressableMessage{
				*m,
				addr,
			}
		}
	}
}

func (s *server) handle(params *Params) {
	for {
		select {
		case am := <-s.incoming:
			m := am.Message
			addr := am.addr

			switch m.Type {
			case MsgConnect:
				// detect duplicate connection request
				duplicate := false
				for _, client := range s.clients {
					if client.addr.String() == addr.String() {
						duplicate = true
						break
					}
				}
				if duplicate {
					continue
				}

				// create new client info
				client := &clientInfo{
					id:      s.lastConnID,
					addr:    addr,
					udpConn: s.udpConn,

					incoming: make(chan *Message, 1024),

					tbuffer: make(map[int]*Message),
					tmsg:    make(chan *Message, 1024),
					tsq:     1,

					rbuffer: make(map[int][]byte),
					rsq:     1,

					timer:   time.Tick(time.Duration(params.EpochMillis) * time.Millisecond),
					retries: params.EpochLimit,
					windows: params.WindowSize,

					status: newStatus(),
				}
				s.clients[s.lastConnID] = client
				s.lastConnID++
				go client.handleClient(s)

				// send ack
				response := NewAck(client.id, 0)
				go WriteMessage(s.udpConn, addr, response)
			default:
				if client, ok := s.clients[m.ConnID]; ok {
					client.incoming <- &m
				}
			}
		case m := <-s.outgoing:
			if client, ok := s.clients[m.ConnID]; ok {
				client.tmsg <- m
			}
		case id := <-s.cls:
			if c, ok := s.clients[id]; ok {
				if c.status.get() == notClosing {
					c.status.set(startClosing)
				}
				delete(s.clients, id)
			}

			if len(s.clients) == 0 && s.status.get() == startClosing {
				s.status.set(handlerClosed)
				return
			}
		}
	}
}

func (c *clientInfo) handleClient(s *server) {
	var epochCount int

	for {
		if c.status.get() == startClosing && len(c.tbuffer) == 0 && len(c.rbuffer) == 0 && len(c.tmsg) == 0 {
			c.status.set(handlerClosed)
			s.cls <- c.id
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
				// save data into buffer
				if m.Size > len(m.Payload) {
					continue
				}
				m.Payload = m.Payload[0:m.Size]

				if _, ok := c.rbuffer[m.SeqNum]; !ok {
					c.rbuffer[m.SeqNum] = m.Payload
				}

				if m.SeqNum == c.rsq {
					i := c.rsq
					for {
						data, ok := c.rbuffer[i]
						if !ok {
							break
						}
						s.rmsg <- &clientData{c.id, data}
						c.rsq++
						delete(c.rbuffer, i)
						i++
					}
				}

				// send ack
				response := NewAck(c.id, m.SeqNum)
				go WriteMessage(s.udpConn, c.addr, response)

			case MsgAck:
				if _, ok := c.tbuffer[m.SeqNum]; ok {
					delete(c.tbuffer, m.SeqNum)
				}
			}

		case <-c.timer:
			epochCount++

			if epochCount == c.retries {
				s.cls <- c.id
				s.err <- &clientError{
					id:  c.id,
					err: fmt.Errorf("[s] client %d: Connection lost", c.id),
				}
				c.status.set(handlerClosed)
				return
			}

			if c.rsq == 1 && len(c.rbuffer) == 0 {
				outMessage := NewAck(c.id, 0)
				go WriteMessage(s.udpConn, c.addr, outMessage)
			}
			for _, outMessage := range c.tbuffer {
				go WriteMessage(s.udpConn, c.addr, outMessage)
			}
		default:
			time.Sleep(time.Nanosecond)

			if c.tsq-minUnAcked < c.windows {
				select {
				case m := <-c.tmsg:
					m.SeqNum = c.tsq

					c.tbuffer[c.tsq] = m
					c.tsq++
					go WriteMessage(s.udpConn, c.addr, m)
				default:
				}
			}

			if c.status.get() == notClosing {
				select {
				case <-s.bcls:
					c.status.set(startClosing)
				default:
				}
			}
		}
	}
}

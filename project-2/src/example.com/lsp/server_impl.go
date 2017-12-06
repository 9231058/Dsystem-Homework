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

	rmsg chan *DataBufferElement

	status           Status
	clientClosedChan chan int
	clientLostChan   chan *ClientLost
}

type clientInfo struct {
	id   int
	addr *net.UDPAddr

	incoming chan *Message

	tbuffer map[int]*Message
	tmsg    chan *Message
	tsq     int

	rbuffer map[int][]byte
	rsq     int

	timer <-chan time.Time

	closingChan chan int
}

type DataBufferElement struct {
	connectionId int
	data         []byte
}

type addressableMessage struct {
	Message
	addr *net.UDPAddr
}

type ClientLost struct {
	connectionId int
	err          error
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

		rmsg: make(chan *DataBufferElement, 10000),

		status:           NotClosing,
		clientClosedChan: make(chan int, 1000),
		clientLostChan:   make(chan *ClientLost, 1000),
	}

	go s.receiver()
	go s.handle(params)

	return s, nil
}

func (s *server) Read() (int, []byte, error) {
	select {
	case element := <-s.rmsg:
		return element.connectionId, element.data, nil
	case lost := <-s.clientLostChan:
		return lost.connectionId, nil, lost.err
	}
}

func (s *server) Write(connID int, payload []byte) error {
	client, ok := s.clients[connID]
	if ok {
		message := NewData(connID, -1, len(payload), payload)
		client.tmsg <- message
	} else {
	}

	return nil
}

func (s *server) CloseConn(connID int) error {
	s.clientClosedChan <- connID

	return nil
}

func (s *server) Close() error {
	s.status = StartClosing

	for {
		if s.status == HandlerClosed {
			s.udpConn.Close()
		}
		if s.status == ConnectionClosed {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *server) receiver() {
	for {
		m, addr, err := ReadMessage(s.udpConn)
		if err != nil {
			if s.status == HandlerClosed {
				s.status = ConnectionClosed
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
					id:   s.lastConnID,
					addr: addr,

					incoming: make(chan *Message, 1024),

					tbuffer: make(map[int]*Message),
					tmsg:    make(chan *Message, 1024),
					tsq:     1,

					rbuffer: make(map[int][]byte),
					rsq:     1,

					timer: time.Tick(time.Duration(params.EpochMillis) * time.Millisecond),

					closingChan: make(chan int, 1000),
				}
				s.clients[s.lastConnID] = client
				s.lastConnID++
				go writeHandlerForClient(s, client, params)

				// send ack
				response := NewAck(client.id, 0)
				go WriteMessage(s.udpConn, addr, response)
			default:
				client, exists := s.clients[m.ConnID]
				if exists {
					client.incoming <- &m
				}
			}

		case connectionId := <-s.clientClosedChan:
			client, exists := s.clients[connectionId]
			if exists {
				client.closingChan <- 1
			}
			delete(s.clients, connectionId)

			if len(s.clients) == 0 && s.status == StartClosing {
				s.status = HandlerClosed
				return
			}
		}
	}
}

func writeHandlerForClient(s *server, c *clientInfo, params *Params) {
	var epochCount int

	for {
		if s.status == StartClosing && len(c.tbuffer) == 0 && len(c.rbuffer) == 0 && len(c.tmsg) == 0 {
			s.clientClosedChan <- c.id
			return
		}

		minUnAcked := c.tsq
		for sq := range c.tbuffer {
			if minUnAcked > sq {
				minUnAcked = sq
			}
		}

		select {
		case <-c.closingChan:
			return

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
						s.rmsg <- &DataBufferElement{c.id, data}
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

			if epochCount == params.EpochLimit {
				s.clientClosedChan <- c.id
				s.clientLostChan <- &ClientLost{c.id, fmt.Errorf("client Lost")}
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

			if c.tsq-minUnAcked < params.WindowSize {
				select {
				case m := <-c.tmsg:
					m.SeqNum = c.tsq

					c.tbuffer[c.tsq] = m
					c.tsq++
					go WriteMessage(s.udpConn, c.addr, m)
				default:
				}
			}
		}
	}
}

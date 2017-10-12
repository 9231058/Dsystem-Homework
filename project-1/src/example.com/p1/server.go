// Implementation of a KeyValueServer. Students should write their code in this file.

package p1

import (
	"bufio"
	"fmt"
	"net"

	"../clist"
)

type keyValueServer struct {
	ln      net.Listener
	res     chan *request
	req     chan *request
	clients *clist.ConcurrentList
}

// New creates and returns (but does not start) a new KeyValueServer.
func New() KeyValueServer {
	return &keyValueServer{
		clients: clist.New(),
		req:     make(chan *request, 500),
		res:     make(chan *request, 500),
	}
}

func (kvs *keyValueServer) Start(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	kvs.ln = ln
	createDB()
	go kvs.dispatch()
	go kvs.listen()
	go kvs.sender()
	return nil
}

func (kvs *keyValueServer) Close() {
	// TODO: implement this!
}

func (kvs *keyValueServer) Count() int {
	return kvs.clients.Len()
}

func (kvs *keyValueServer) listen() {
	for {
		conn, err := kvs.ln.Accept()
		if err != nil {
			return
		}

		c := &client{
			conn:   conn,
			writer: bufio.NewWriter(conn),
			reader: bufio.NewReader(conn),
		}
		kvs.clients.PushBack(c)
		go kvs.receiver(c)
	}

}

func (kvs *keyValueServer) sender() {
	for {
		select {
		case r := <-kvs.res:
			cs := kvs.clients.Iter()
			for c := range cs {
				c.Value.(*client).writer.WriteString(fmt.Sprintf("%s,%s\n", r.key, r.value))
				c.Value.(*client).writer.Flush()

			}
		default:
			continue
		}
	}
}

func (kvs *keyValueServer) receiver(c *client) {

	defer func() {
		kvs.clients.Remove(c)
	}()
	for {
		var command, key string
		var value []byte

		buf, err := c.reader.ReadBytes(',')
		if err != nil {
			return
		}
		command = string(buf[:len(buf)-1])
		if command == "get" {
			buf, err := c.reader.ReadBytes('\n')
			if err != nil {
				return
			}
			key = string(buf[:len(buf)-1])
			kvs.req <- &request{
				command: "get",
				key:     key,
			}
		} else if command == "set" {
			buf, err := c.reader.ReadBytes(',')
			if err != nil {
				return
			}
			key = string(buf[:len(buf)-1])
			buf, err = c.reader.ReadBytes('\n')
			if err != nil {
				return
			}
			value = buf[:len(buf)-1]
			kvs.req <- &request{
				command: "set",
				key:     key,
				value:   value,
			}
		}
	}
}

func (kvs *keyValueServer) dispatch() {
	for {
		select {
		case r := <-kvs.req:
			if r.command == "get" {
				kvs.res <- &request{
					key:   r.key,
					value: get(r.key),
				}
			}
			if r.command == "set" {
				set(r.key, r.value)
			}
		}
	}
}

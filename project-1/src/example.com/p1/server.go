// Implementation of a KeyValueServer. Students should write their code in this file.

package p1

import (
	"bufio"
	"fmt"
	"net"
	"runtime"

	"../clist"
)

type keyValueServer struct {
	ln      net.Listener
	clients *clist.ConcurrentList
}

// New creates and returns (but does not start) a new KeyValueServer.
func New() KeyValueServer {
	return &keyValueServer{
		clients: clist.New(),
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
	return nil
}

func (kvs *keyValueServer) Close() {
	kvs.ln.Close()
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
			req:    make(chan *request, 500),
			res:    make(chan *request, 500),
		}
		kvs.clients.PushBack(c)
		go kvs.sender(c)
		go kvs.receiver(c)
	}

}

func (kvs *keyValueServer) sender(c *client) {
	for {
		r := <-c.res
		c.writer.WriteString(fmt.Sprintf("%s,%s\n", r.key, r.value))
		c.writer.Flush()
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
			c.req <- &request{
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
			c.req <- &request{
				command: "set",
				key:     key,
				value:   value,
			}
		}
	}
}

func (kvs *keyValueServer) dispatch() {
	for {
		var res []*request
		for c := range kvs.clients.Iter() {
			select {
			case r := <-c.Value.(*client).req:
				if r.command == "get" {
					res = append(res, &request{
						key:   r.key,
						value: get(r.key),
					})
				}
				if r.command == "set" {
					set(r.key, r.value)
				}
			default:
				continue
			}
		}
		runtime.Gosched()
		if len(res) != 0 {
			for c := range kvs.clients.Iter() {
				for _, r := range res {
					c.Value.(*client).res <- r
				}
			}

		}
	}
}

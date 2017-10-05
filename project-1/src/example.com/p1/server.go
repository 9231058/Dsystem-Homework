// Implementation of a KeyValueServer. Students should write their code in this file.

package p1

import (
	"bufio"
	"fmt"
	"net"
)

type request struct {
	command string
	key     string
	value   []byte
	writer  *bufio.Writer
}

type keyValueServer struct {
	ln      net.Listener
	ch      chan *request
	clients int
}

// New creates and returns (but does not start) a new KeyValueServer.
func New() KeyValueServer {
	return &keyValueServer{
		clients: 0,
		ch:      make(chan *request),
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
	// TODO: implement this!
}

func (kvs *keyValueServer) Count() int {
	return kvs.clients
}

func (kvs *keyValueServer) listen() {
	for {
		conn, err := kvs.ln.Accept()
		if err != nil {
			return
		}
		kvs.clients++
		go kvs.handle(conn)
	}

}

func (kvs *keyValueServer) handle(conn net.Conn) {
	cr := bufio.NewReader(conn)
	cw := bufio.NewWriter(conn)
	for {
		var command, key string
		var value []byte

		buf, err := cr.ReadBytes(',')
		if err != nil {
			kvs.clients--
			return
		}
		command = string(buf[:len(buf)-1])
		if command == "get" {
			buf, err := cr.ReadBytes('\n')
			if err != nil {
				kvs.clients--
				return
			}
			key = string(buf[:len(buf)-1])
			kvs.ch <- &request{
				command: "get",
				key:     key,
				writer:  cw,
			}
		} else if command == "set" {
			buf, err := cr.ReadBytes(',')
			if err != nil {
				kvs.clients--
				return
			}
			key = string(buf[:len(buf)-1])
			buf, err = cr.ReadBytes('\n')
			if err != nil {
				kvs.clients--
				return
			}
			value = buf[:len(buf)-1]
			kvs.ch <- &request{
				command: "set",
				key:     key,
				value:   value,
				writer:  cw,
			}
		}
	}
}

func (kvs *keyValueServer) dispatch() {
	for {
		select {
		case r := <-kvs.ch:
			if r.command == "get" {
				r.writer.WriteString(fmt.Sprintf("%s,%s\n", r.key, get(r.key)))
				r.writer.Flush()
			}
			if r.command == "set" {
				set(r.key, r.value)
			}
		}
	}
}

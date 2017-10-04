// Implementation of a KeyValueServer. Students should write their code in this file.

package p1

import (
	"fmt"
	"net"
)

type keyValueServer struct {
	ln      net.Listener
	clients int
}

// New creates and returns (but does not start) a new KeyValueServer.
func New() KeyValueServer {
	return &keyValueServer{
		clients: 0,
	}
}

func (kvs *keyValueServer) Start(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	kvs.ln = ln
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
	for {
		var command, key string
		var value []byte
		fmt.Fscanf(conn, "%s, %s, %s", &command, &key, value)
		if command == "set" {
			set(key, value)
		} else if command == "get" {
			get(key)
		}
	}
}

// TODO: add additional methods/functions below!

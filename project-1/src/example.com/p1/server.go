// Implementation of a KeyValueServer. Students should write their code in this file.

package p1

import (
	"fmt"
	"net"
)

type keyValueServer struct {
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
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		kvs.clients++
		go kvs.handle(conn)
	}
	return nil
}

func (kvs *keyValueServer) Close() {
	// TODO: implement this!
}

func (kvs *keyValueServer) Count() int {
	return kvs.clients
}

func (kvs *keyValueServer) handle(conn net.Conn) {
	for {
		var command, key, value string
		fmt.Fscanf(conn, "%s, %s, %s", &command, &key, &value)
	}
}

// TODO: add additional methods/functions below!
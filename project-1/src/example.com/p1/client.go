package p1

import (
	"bufio"
	"net"
)

type client struct {
	conn   net.Conn
	writer *bufio.Writer
	reader *bufio.Reader
	res    chan *request
	req    chan *request
}

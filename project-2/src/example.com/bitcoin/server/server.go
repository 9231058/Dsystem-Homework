package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	".."
	"../../lsp"
)

type miner struct {
	id int
}

type server struct {
	lspServer lsp.Server
	miners    *list.List
}

func startServer(port int) (*server, error) {
	lspServer, err := lsp.NewServer(port, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	s := &server{
		lspServer: lspServer,
		miners:    list.New(),
	}

	return s, nil
}

var logf *log.Logger

func main() {
	// You may need a logger for debug purpose
	const (
		name = "log.txt"
		flag = os.O_RDWR | os.O_CREATE
		perm = os.FileMode(0666)
	)

	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return
	}
	defer file.Close()

	logf = log.New(file, "", log.Lshortfile|log.Lmicroseconds)
	// Usage: LOGF.Println() or LOGF.Printf()

	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <port>", os.Args[0])
		return
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Port must be a number:", err)
		return
	}

	srv, err := startServer(port)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("Server listening on port", port)

	defer srv.lspServer.Close()

	// waiting on requests
	for {
		id, b, err := srv.lspServer.Read()
		if err != nil {
			continue
		}
		var m bitcoin.Message
		err = json.Unmarshal(b, &m)
		if err != nil {
			continue
		}
		switch m.Type {
		case bitcoin.Join:
			m := &miner{
				id: id,
			}
			logf.Printf("Miner joined [%v]", m)
			srv.miners.PushBack(m)
		}
	}
}

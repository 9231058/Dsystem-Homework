package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	".."
	"../../lsp"
)

type task struct {
	id int

	minHash  uint64
	minNonce uint64
	miners   int
}

type request struct {
	id int

	lower uint64
	upper uint64
	data  string
}

type server struct {
	lspServer lsp.Server

	requests chan request

	freeMiners map[int]bool
	busyMiners map[int]int

	tasks map[int]*task
}

func startServer(port int) (*server, error) {
	lspServer, err := lsp.NewServer(port, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	s := &server{
		lspServer: lspServer,

		requests: make(chan request, 1024),

		freeMiners: make(map[int]bool),
		busyMiners: make(map[int]int),
		tasks:      make(map[int]*task),
	}

	return s, nil
}

func (srv *server) receive() {
	for {
		id, b, err := srv.lspServer.Read()
		if err != nil {
			continue
		}

		var m bitcoin.Message
		json.Unmarshal(b, &m)

		switch m.Type {
		case bitcoin.Join:
			logf.Printf("Miner joined [%v]", id)
			srv.freeMiners[id] = true
		case bitcoin.Request:
			minersNum := 2

			srv.requests <- request{
				upper: m.Upper,
				lower: m.Lower,
				data:  m.Data,
			}

			srv.tasks[id] = &task{
				miners: minersNum,
			}

			if len(srv.freeMiners) >= minersNum {
				var i uint64
				for mid := range srv.freeMiners {
					if i > uint64(minersNum) {
						break
					}
					r := bitcoin.NewRequest(m.Data, i*m.Upper/uint64(minersNum), (i+1)*m.Upper/uint64(minersNum))
					b, _ := json.Marshal(r)
					srv.lspServer.Write(mid, b)

					i++
					delete(srv.freeMiners, mid)
					srv.busyMiners[mid] = id
				}
			}

		case bitcoin.Result:
			cid := srv.busyMiners[id]
			task := srv.tasks[cid]
			delete(srv.busyMiners, id)
			srv.freeMiners[id] = true

			task.miners--
			if task.minHash == 0 || task.minHash < m.Hash {
				task.minHash = m.Hash
				task.minNonce = m.Nonce
			}
			if task.miners == 0 {
				r := bitcoin.NewResult(task.minHash, task.minNonce)
				b, _ := json.Marshal(r)
				srv.lspServer.Write(cid, b)
			}
		}

	}

}

func (srv *server) schedule() {
	timer := time.Tick(100)
	for {
		select {
		case <-timer:
		}
	}
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

	go srv.schedule()
	srv.receive()
}

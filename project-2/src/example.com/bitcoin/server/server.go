package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
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

type result struct {
	id int

	nonce uint64
	hash  uint64
}

type server struct {
	lspServer lsp.Server

	requests chan request
	results  chan result

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
		results:  make(chan result, 1024),

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
			srv.requests <- request{
				id: id,

				upper: m.Upper,
				lower: m.Lower,
				data:  m.Data,
			}

		case bitcoin.Result:
			srv.results <- result{
				id: id,

				hash:  m.Hash,
				nonce: m.Nonce,
			}
		}

	}

}

func (srv *server) schedule() {
	timer := time.Tick(100)
	for {
		select {
		case <-timer:
			if len(srv.freeMiners) > 0 {
				select {
				case r := <-srv.requests:
					minersNum := int(math.Floor(math.Min(float64(len(srv.freeMiners)), math.Log(float64(r.upper)))))

					srv.tasks[r.id] = &task{
						miners: minersNum,
					}

					var i uint64
					for mid := range srv.freeMiners {
						logf.Println(mid)
						if i > uint64(minersNum) {
							break
						}
						m := bitcoin.NewRequest(r.data, i*r.upper/uint64(minersNum), (i+1)*r.upper/uint64(minersNum))
						logf.Println(m)
						b, _ := json.Marshal(m)
						err := srv.lspServer.Write(mid, b)
						if err != nil {
							logf.Println(err)
						}

						i++
						delete(srv.freeMiners, mid)
						srv.busyMiners[mid] = r.id
					}
				default:
					continue

				}
			}
		case r := <-srv.results:
			cid := srv.busyMiners[r.id]
			task := srv.tasks[cid]
			delete(srv.busyMiners, r.id)
			srv.freeMiners[r.id] = true

			task.miners--
			if task.minHash == 0 || task.minHash < r.hash {
				task.minHash = r.hash
				task.minNonce = r.nonce
			}
			if task.miners == 0 {
				r := bitcoin.NewResult(task.minHash, task.minNonce)
				logf.Println(r)
				b, _ := json.Marshal(r)
				err := srv.lspServer.Write(cid, b)
				if err != nil {
					logf.Println(err)
				}
			}

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

package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"

	".."
	"../../lsp"
)

type task struct {
	lost bool

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

type miner struct {
	clientID int
	data     string
	upper    uint64
	lower    uint64
}

type server struct {
	lspServer lsp.Server

	joins    chan int
	requests chan request
	results  chan result
	errors   chan int

	freeMiners      *list.List     // list of free miners identification
	miners          map[int]*miner // map between miners information and their identification
	pendingRequests *list.List     // list of pending requests

	tasks map[int]*task
}

func min(a, b uint64) uint64 {
	if b > a {
		return a
	}
	return b
}

func startServer(port int) (*server, error) {
	lspServer, err := lsp.NewServer(port, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	s := &server{
		lspServer: lspServer,

		joins:    make(chan int, 1024),
		requests: make(chan request, 1024),
		results:  make(chan result, 1024),
		errors:   make(chan int, 1024),

		freeMiners:      list.New(),
		miners:          make(map[int]*miner),
		pendingRequests: list.New(),

		tasks: make(map[int]*task),
	}

	return s, nil
}

func (srv *server) receive() {
	for {
		id, b, err := srv.lspServer.Read()
		if err != nil {
			// connection lost event occurred
			srv.errors <- id
			continue
		}

		var m bitcoin.Message
		json.Unmarshal(b, &m)

		switch m.Type {
		case bitcoin.Join:
			srv.joins <- id

		case bitcoin.Request:
			logf.Println(id, m)
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
	trigger := make(chan struct{}, 1024)
	for {
		select {
		case r := <-srv.requests:
			var minersNum = 1
			if r.upper > 10000 {
				minersNum = int(math.Floor(math.Log10(float64(r.upper))))
			}
			step := uint64(math.Ceil(float64(r.upper) / float64(minersNum)))

			srv.tasks[r.id] = &task{
				miners: minersNum,
			}

			for i := 0; i < minersNum; i++ {
				srv.pendingRequests.PushBack(request{
					id:    r.id,
					data:  r.data,
					lower: uint64(i) * step,
					upper: min(r.upper, uint64(i+1)*step),
				})
			}
			trigger <- struct{}{}
		case <-trigger:
			// removes dead miner
			for me := srv.freeMiners.Front(); me != nil; me = me.Next() {
				if srv.miners[me.Value.(int)] == nil {
					srv.freeMiners.Remove(me)
				}
			}

			// is there anyone who can do some mining :P
			if srv.freeMiners.Len() > 0 {
				if srv.pendingRequests.Len() > 0 {
					re := srv.pendingRequests.Front()
					r := re.Value.(request)

					logf.Println(r)

					me := srv.freeMiners.Front()
					mid := me.Value.(int)

					m := bitcoin.NewRequest(r.data, r.lower, r.upper)
					b, _ := json.Marshal(m)
					err := srv.lspServer.Write(mid, b)
					if err != nil {
						srv.errors <- mid
						continue
					}

					srv.pendingRequests.Remove(re)
					srv.freeMiners.Remove(me)
					srv.miners[mid] = &miner{
						clientID: r.id,
						upper:    m.Upper,
						lower:    m.Lower,
						data:     m.Data,
					}
					logf.Println("***", mid, srv.miners[mid])
				}
			}
		case id := <-srv.joins:
			srv.freeMiners.PushBack(id)
			srv.miners[id] = &miner{
				clientID: 0,
			}
			trigger <- struct{}{}

		case r := <-srv.results:
			miner := srv.miners[r.id]
			if miner == nil || miner.clientID == 0 {
				continue
			}

			task := srv.tasks[miner.clientID]

			logf.Println("Result", miner.clientID, task)

			task.miners--
			if task.minHash == 0 || task.minHash > r.hash {
				task.minHash = r.hash
				task.minNonce = r.nonce
			}
			if task.miners == 0 {
				r := bitcoin.NewResult(task.minHash, task.minNonce)
				if !task.lost {
					b, _ := json.Marshal(r)
					srv.lspServer.Write(miner.clientID, b)
				}
				delete(srv.tasks, miner.clientID)
			}
			srv.miners[r.id].clientID = 0
			srv.freeMiners.PushBack(r.id)
			trigger <- struct{}{}

		case e := <-srv.errors:
			if mn := srv.miners[e]; mn != nil {
				if mn.clientID != 0 {
					logf.Println("Busy miner", e)

					srv.pendingRequests.PushFront(request{
						id:    mn.clientID,
						upper: mn.upper,
						lower: mn.lower,
						data:  mn.data,
					})

					delete(srv.miners, e)
					trigger <- struct{}{}
				} else {
					logf.Println("Free miner", e)
					delete(srv.miners, e)
				}
			} else if task := srv.tasks[e]; task != nil {
				logf.Println("Client", e)
				task.lost = true
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

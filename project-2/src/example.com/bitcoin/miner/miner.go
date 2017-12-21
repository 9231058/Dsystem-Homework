package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"

	".."
	"../../lsp"
)

type result struct {
	nonce, hash uint64
}

func sendJoinRequest(c lsp.Client) error {
	m := bitcoin.NewJoin()
	b, _ := json.Marshal(m)
	err := c.Write(b)
	if err != nil {
		return err
	}
	return nil
}

// Attempt to connect miner as a client to the server.
func joinWithServer(hostport string) (lsp.Client, error) {
	c, err := lsp.NewClient(hostport, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	err = sendJoinRequest(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func main() {
	const (
		name = "log.txt"
		flag = os.O_RDWR | os.O_CREATE
		perm = os.FileMode(0666)
	)

	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return
	}

	LOGF := log.New(file, "", log.Lshortfile|log.Lmicroseconds)

	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport>", os.Args[0])
		return
	}

	hostport := os.Args[1]
	miner, err := joinWithServer(hostport)
	if err != nil {
		fmt.Println("Failed to join with server:", err)
		return
	}

	defer miner.Close()

	// watiing on requests
	for {
		b, err := miner.Read()
		if err != nil {
			return
		}
		var m bitcoin.Message
		json.Unmarshal(b, &m)

		if m.Type == bitcoin.Request {
			w := int(math.Floor(math.Log(float64(m.Upper - m.Lower))))
			s := uint64((m.Upper - m.Lower) / uint64(w))
			rc := make(chan result, 1)

			LOGF.Println("Request: ", w, s)

			for i := 0; i < w; i++ {
				go func(lower, upper uint64) {
					var minHash uint64
					var minNonce uint64

					LOGF.Println("Bound: ", lower, upper)

					for nonce := lower; nonce <= upper; nonce++ {
						hash := bitcoin.Hash(m.Data, nonce)
						if minHash == 0 || minHash > hash {
							minHash = hash
							minNonce = nonce
						}
					}

					rc <- result{minNonce, minHash}
				}(m.Lower+uint64(i)*s, m.Lower+uint64(i+1)*s)
			}

			var minHash uint64
			var minNonce uint64

			for w > 0 {
				r := <-rc
				w--
				if minHash == 0 || minHash > r.hash {
					minHash = r.hash
					minNonce = r.nonce
				}
			}

			r := bitcoin.NewResult(minHash, minNonce)
			b, _ := json.Marshal(r)
			err := miner.Write(b)

			if err != nil {
				return
			}

		}
	}
}

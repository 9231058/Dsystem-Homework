package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	".."
	"../../lsp"
)

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

	// send join request for time out avoidance
	go func() {
		timer := time.Tick(time.Duration(lsp.DefaultEpochMillis) * time.Millisecond)
		for {
			<-timer
			sendJoinRequest(miner)
		}
	}()

	// watiing on requests
	for {
		b, err := miner.Read()
		if err != nil {
			fmt.Println(err)
			return
		}
		var m bitcoin.Message
		json.Unmarshal(b, &m)

		if m.Type == bitcoin.Request {
			var minHash uint64
			var minNonce uint64

			for nonce := m.Lower; nonce <= m.Upper; nonce++ {
				hash := bitcoin.Hash(m.Data, nonce)
				if minHash == 0 || minHash > hash {
					minHash = hash
					minNonce = nonce
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

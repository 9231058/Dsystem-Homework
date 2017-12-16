package main

import (
	"encoding/json"
	"fmt"
	"os"

	".."
	"../../lsp"
)

// Attempt to connect miner as a client to the server.
func joinWithServer(hostport string) (lsp.Client, error) {
	c, err := lsp.NewClient(hostport, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	m := bitcoin.NewJoin()
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	err = c.Write(b)
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

	// TODO: implement this!
}

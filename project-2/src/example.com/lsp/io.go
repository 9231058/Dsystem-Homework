package lsp

import (
	"encoding/json"
	"log"

	net "../lspnet"
)

// ReadMessage receives message from given connection and de-serializes it.
func ReadMessage(connection *net.UDPConn) (*Message, *net.UDPAddr, error) {
	packet := make([]byte, 2000)

	n, addr, err := connection.ReadFromUDP(packet)
	if err != nil {
		log.Fatalf("ReadMessage Error: %s\n", err)
		return nil, addr, err
	}

	packet = packet[0:n]
	var message Message
	err = json.Unmarshal(packet, &message)
	if err != nil {
		log.Fatalf("ReadMessage Error: %s\n", err)
		return nil, addr, err
	}
	return &message, addr, nil

}

// WriteMessage serializes given message and send it into by given connection.
func WriteMessage(connection *net.UDPConn, addr *net.UDPAddr, message *Message) error {
	packet, err := json.Marshal(message)
	if err != nil {
		log.Fatal("WriteMessage Error: %s\n", err)
		return err
	}

	if addr != nil {
		_, err = connection.WriteToUDP(packet, addr)
	} else {
		_, err = connection.Write(packet)
	}
	if err != nil {
		log.Fatal("WriteMessage Error: %s\n", err)
		return err
	}

	return nil
}

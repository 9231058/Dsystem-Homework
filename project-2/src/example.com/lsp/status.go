package lsp

// Status represents server and connection state
type Status int

const (
	NotClosing Status = iota
	StartClosing
	HandlerClosed
	ConnectionClosed
	Closed
)

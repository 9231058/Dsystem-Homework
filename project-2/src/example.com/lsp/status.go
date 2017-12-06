package lsp

import "sync"

type status struct {
	status int
	lock   *sync.RWMutex
}

const (
	notClosing int = iota
	startClosing
	handlerClosed
	connectionClosed
	closed
)

func newStatus() *status {
	return &status{
		status: notClosing,
		lock:   new(sync.RWMutex),
	}
}

func (s *status) set(status int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.status = status
}

func (s *status) get() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.status
}

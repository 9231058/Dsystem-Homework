package lsp

import "sync"

type status struct {
	s    int
	lock *sync.RWMutex
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
		s:    notClosing,
		lock: new(sync.RWLock),
	}
}

func (s *status) set(s int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.s = s
}

func (s *status) get() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.s
}

package main

import (
	"container/list"
	"sync"
)

// BufferedChannel provides go channel like interface with unlimited storage
type BufferedChannel struct {
	m *sync.Mutex
	l *list.List
	c *sync.Cond
}

// New Creates new buffer channel
func New() *BufferedChannel {
	m := new(sync.Mutex)
	return &BufferedChannel{
		m: m,
		l: list.New(),
		c: sync.NewCond(m),
	}
}

// Append adds given data at end of channel
func (b *BufferedChannel) Append(v interface{}) {
	b.m.Lock()
	defer b.m.Unlock()

	b.l.PushBack(v)
	b.c.Signal()

}

// Remove removes first element of list synchronously
func (b *BufferedChannel) Remove() interface{} {
	b.m.Lock()
	defer b.m.Unlock()

	for b.l.Len() == 0 {
		b.c.Wait()
	}

	v := b.l.Front()
	b.l.Remove(v)

	return v.Value
}

// AsyncRemove removes first element of list asynchronously
func (b *BufferedChannel) AsyncRemove() interface{} {
	b.m.Lock()
	defer b.m.Unlock()

	for b.l.Len() == 0 {
		return nil
	}

	v := b.l.Front()
	b.l.Remove(v)

	return v.Value
}

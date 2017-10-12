package clist

import (
	"container/list"
)

type ConcurrentList struct {
	mutex chan int
	items *list.List
}

// New returns an initialized list.
func New() *ConcurrentList {
	return &ConcurrentList{
		mutex: make(chan int, 1),
		items: list.New(),
	}
}

// PushBack inserts a new element e with value v at the back of list l and returns e.
func (cl *ConcurrentList) PushBack(v interface{}) *list.Element {
	cl.mutex <- 1
	defer func() { <-cl.mutex }()

	return cl.items.PushBack(v)
}

// Len returns the number of elements of list l. The complexity is O(1).
func (cl *ConcurrentList) Len() int {
	cl.mutex <- 1
	defer func() { <-cl.mutex }()

	return cl.items.Len()
}

// Remove removes e from l if e is an element of list l. It returns the element value e.Value.
func (cl *ConcurrentList) Remove(e *list.Element) interface{} {
	cl.mutex <- 1
	defer func() { <-cl.mutex }()

	return cl.items.Remove(e)
}

// Iter iterates over the items in the concurrent list
func (cl *ConcurrentList) Iter() <-chan *list.Element {
	c := make(chan *list.Element)

	go func() {
		cl.mutex <- 1
		defer func() { <-cl.mutex }()
		for e := cl.items.Front(); e != nil; e = e.Next() {
			c <- e
		}
		close(c)
	}()

	return c
}

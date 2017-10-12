package clist

import (
	"container/list"
)

// ConcurrentList represents a synchronized doubly linked list. The zero value for List is an empty list ready to use.
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

// Remove removes first occurrence of v from l if v is an element of list l. It returns the element value v.
func (cl *ConcurrentList) Remove(v interface{}) interface{} {
	cl.mutex <- 1
	defer func() { <-cl.mutex }()

	for e := cl.items.Front(); e != nil; e = e.Next() {
		if e.Value == v {
			return cl.items.Remove(e)
		}
	}

	return nil
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

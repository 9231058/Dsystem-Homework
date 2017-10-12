package clist

import (
	"container/list"
)

type ConcurrentList struct {
	mutex chan int
	items *list.List
}

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

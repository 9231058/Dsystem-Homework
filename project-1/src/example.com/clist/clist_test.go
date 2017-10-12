package clist

import (
	"sync"
	"testing"
)

func TestAppend(t *testing.T) {
	var wg sync.WaitGroup

	cl := New()
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cl.PushBack(1)
	}(&wg)
	wg.Add(1)

	cl.PushBack(2)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		t.Logf("%d\n", cl.Len())
	}(&wg)
	wg.Add(1)

	wg.Wait()
}

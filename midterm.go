/*
 * +===============================================
 * | Author:        Parham Alvani <parham.alvani@gmail.com>
 * |
 * | Creation Date: 29-11-2017
 * |
 * | File Name:     midterm.go
 * +===============================================
 */

package main

import "fmt"

func fibonacci(c chan int) {
	x, y := 0, 1
	for i := 0; i < cap(c); i++ {
		c <- x
		x, y = y, x+y
	}
	close(c)
}

func main() {
	c := make(chan int, 6)
	fibonacci(c)
	for x := range c {
		fmt.Println(x)
	}
}

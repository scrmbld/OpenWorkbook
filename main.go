package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/scrmbld/course-site/procweb"
)

// start the lua program
// set up the pipes
// scan the pipes
// wait for messages from the sockets

var wg = sync.WaitGroup{}

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:4400")
	if err != nil {
		panic(err)
	}

	conn, err := listener.Accept()
	defer conn.Close() // not sure how this interacts with panic
	if err != nil {
		panic(err)
	}

	procweb.NewInstance("lua/test.lua", conn)
	fmt.Println("main: done")
}

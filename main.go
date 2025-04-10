package main

import (
	"fmt"
	"net"

	"gihub.com/scrmbld/course-site/procWeb"
)

// start the lua program
// set up the pipes
// scan the pipes
// wait for messages from the sockets

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:4400")
	if err != nil {
		panic(err)
	}

	sock, err := listener.Accept()
	if err != nil {
		panic(err)
	}

	msgChan := make(chan procWeb.ProcMessage, 5)
	go procWeb.ScanProcConnection(sock, msgChan)
	for msg := range msgChan {
		fmt.Println(msg)
		fmt.Println("======")
	}
}

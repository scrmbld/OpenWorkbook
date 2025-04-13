package procweb

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// run a new program with CLI I/O being sent over the network
func NewInstance(program string, sock net.Conn) {
	defer sock.Close()
	var wg sync.WaitGroup

	// these consume from the process
	stdinChan := make(chan []byte, 32)
	stdoutChan := make(chan []byte, 32)
	stderrChan := make(chan []byte, 32)
	// these consume from the socket
	incomingMsgChan := make(chan ProcMessage, 32)

	// scan the socket level IO
	wg.Add(3)
	go ScanProcConnection(sock, incomingMsgChan) // includes stdin, but possibly also other things
	go SendProcConnection(sock, stdoutChan, "stdout")
	go SendProcConnection(sock, stderrChan, "stdin")

	// consume the incoming messages and pass new messages to the right places
	// for example, forward the body of stdin messages to stdinChan
	go func() {
		for {
			msg := <-incomingMsgChan
			switch msg.Category {
			case "stdin":
				stdinChan <- []byte(msg.Body)
			default:
				log.Printf("unsupported message category: %s", msg.Category)
			}
		}
	}()

	// run the program
	wg.Add(1)
	go RunLua(program, stdinChan, stdoutChan, stderrChan, &wg)

	// wg.Add(2)
	// go printOutput(stdoutChan, &wg)
	// go printOutput(stderrChan, &wg)

	wg.Wait()
	fmt.Println("program done")
}

func printOutput(outChan chan []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	defer fmt.Println("done reading")
	for msg := range outChan {
		fmt.Print(string(msg))
	}
}

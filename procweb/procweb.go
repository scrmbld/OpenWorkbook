package procweb

import (
	"context"
	"log"
	"net"
	"os"
	"sync"
)

var ProcLog *log.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmsgprefix|log.Llongfile)

// run a new program with CLI I/O being sent over the network
func NewInstance(program string, sock net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())

	defer sock.Close()

	var wg sync.WaitGroup

	// these hold messages I/O for the lua process
	stdinChan := make(chan []byte, 8)
	stdoutChan := make(chan []byte, 8)
	stderrChan := make(chan []byte, 8)

	// scan our process I/O
	// wg.Add(3)
	incomingMsgChan := ScanProcConnection(ctx, cancel, sock)
	SendProcConnection(ctx, cancel, sock, stdoutChan, "stdout")
	SendProcConnection(ctx, cancel, sock, stderrChan, "stderr")

	// consume the incoming messages and pass new messages to the right places
	// for example, forward the body of stdin messages to stdinChan
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-incomingMsgChan:
				ProcLog.Println(msg)
				switch msg.Category {
				case "stdin":
					stdinChan <- []byte(msg.Body)
					ProcLog.Println(msg)
				default:
					ProcLog.Printf("unsupported message category: %s", msg.Category)
				}
			}
		}
	}()

	// run the program
	wg.Add(1)
	go RunLua(ctx, cancel, program, stdinChan, stdoutChan, stderrChan, &wg)

	wg.Wait()
	ProcLog.Println("program done")
}

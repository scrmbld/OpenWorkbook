package procweb

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var ProcLog *log.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmsgprefix|log.Llongfile)

// run a new program with CLI I/O being sent over the network
func NewInstance(program string, ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	var mtx sync.Mutex

	var wg sync.WaitGroup

	// these hold messages I/O for the lua process
	stdinChan := make(chan []byte, 8)
	stdoutChan := make(chan ProcMessage, 8)
	stderrChan := make(chan ProcMessage, 8)

	// scan our process I/O
	// wg.Add(3)
	incomingMsgChan := ScanProcConnection(ctx, cancel, ws, &mtx)
	SendProcConnection(ctx, cancel, ws, &mtx, stdoutChan, "stdout")
	SendProcConnection(ctx, cancel, ws, &mtx, stderrChan, "stderr")

	// consume the incoming messages and pass new messages to the right places
	// for example, forward the body of stdin messages to stdinChan
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-incomingMsgChan:
				switch msg.Category {
				case "stdin":
					stdinChan <- []byte(msg.Body)
					ProcLog.Println(msg)
				case "EOF":
					// end stdin, no more input
					ProcLog.Println("received EOF")
					close(stdinChan)
					return
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

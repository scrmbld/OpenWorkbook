package procweb

import (
	"bytes"
	"context"
	"log"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var ProcLog *log.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmsgprefix|log.Llongfile)

// run a new program with CLI I/O being sent over the network
func NewInstance(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	var mtx sync.Mutex

	var wg sync.WaitGroup

	// read the program
	var prog bytes.Buffer
	for {
		var msg ProcMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			shutdownWs(ws, &mtx)
			ProcLog.Print("error reading program", err)
			return
		}
		if msg.Category == "EOF" {
			break
		}
		prog.WriteString(msg.Body)
	}

	ProcLog.Println("program:", prog.String())

	// write the program to a temporary file
	// err := os.Mkdir("/temp/lua", os.FileMode(700))
	// if err != nil {
	// 	shutdownWs(ws, &mtx)
	// 	ProcLog.Print("failed to make directory for program file")
	// 	return
	// }
	// err = os.WriteFile("/temp/lua/prog.lua", prog.Bytes(), os.FileMode(660))
	// if err != nil {
	// 	shutdownWs(ws, &mtx)
	// 	ProcLog.Print("failed to write program to file")
	// 	return
	// }

	// these hold messages I/O for the lua process
	stdinChan := make(chan []byte, 8)
	stdoutChan := make(chan ProcMessage, 8)
	stderrChan := make(chan ProcMessage, 8)

	// scan our process I/O
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
					if msg.Body == "stdin" {
						// end stdin, no more input
						ProcLog.Println("received EOF")
						close(stdinChan)
					}
					return
				default:
					ProcLog.Printf("unsupported message category: %s", msg.Category)
				}
			}
		}
	}()

	// run the program
	wg.Add(1)
	go RunLua(ctx, cancel, "cmd/procweb/test_lua/echo.lua", stdinChan, stdoutChan, stderrChan, &wg)

	wg.Wait()
	ProcLog.Println("program done")
}

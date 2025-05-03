package procweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"

	"github.com/gorilla/websocket"
)

var ProcLog *log.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmsgprefix|log.Llongfile)

// a type representing the json messages sent between the client/code instance websocket
type ProcMessage struct {
	Category string `json:"category"`
	Body     string `json:"body"`
}

// helper functions
// =====================================

// a helper function for closing a websocket
func shutdownWs(ws *websocket.Conn, mtx *sync.Mutex) {
	mtx.Lock()
	err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	mtx.Unlock()
	if err != nil {
		ProcLog.Print(err)
		ws.Close()
	}
}

// convert a json byte slice into a ProcMessage
func jsonFromMsg(msg ProcMessage) ([]byte, error) {
	result, err := json.Marshal(msg)
	if err != nil {
		return []byte{}, err
	}
	return result, nil
}

// process i/o
// =====================================

// write messages to the subprocess stdin, reading from inChan
func inScanner(ctx context.Context,
	cancel context.CancelFunc,
	pipe io.WriteCloser,
	inChan chan []byte,
) {
	defer pipe.Close()
	defer ProcLog.Println("closing stdin pipe")
ScannerLoop:
	for {
		select {
		case <-ctx.Done():
			ProcLog.Println("write to stdin cancelled")
			cancel()
			return

		case msg, ok := <-inChan:
			if ok == false {
				ProcLog.Println("inChan closed")
				return
			}
			ProcLog.Println(string(msg))
			_, err := pipe.Write(msg)
			if err != nil {
				if errors.Is(err, fs.ErrClosed) {
					ProcLog.Println("stdin: closed")
					break ScannerLoop
				}
				ProcLog.Fatal("stdin: ", err)
			}
		}
	}
}

// websocket communication
// =====================================

// read from a subprocess output pipe, and write them to outChan
func outScanner(ctx context.Context,
	cancel context.CancelFunc,
	pipe io.ReadCloser,
	outChan chan ProcMessage,
	name string,
) {
	defer pipe.Close()
	defer close(outChan)

	for {
		msg := make([]byte, 2048)
		n, err := pipe.Read(msg)
		if err != nil {
			// these two branches are non-error end states, so don't cancel
			if errors.Is(err, fs.ErrClosed) {
				ProcLog.Println(name, "pipe closed")
				return
			}
			if errors.Is(err, io.EOF) {
				ProcLog.Println(name, "pipe EOF")
				return
			}
			// something bad happened, shut it all down
			ProcLog.Println(name, err)
			cancel()
			return
		}

		select {
		case <-ctx.Done():
			ProcLog.Println(name, "cancelled")
			return
		case outChan <- ProcMessage{Category: name, Body: string(msg[:n])}:
			continue
		}
	}
}

// starts a new goroutine that reads from the socket and writes to the returned channel
func ScanProcConnection(
	ctx context.Context,
	cancel context.CancelFunc,
	ws *websocket.Conn,
	mtx *sync.Mutex,
) <-chan ProcMessage {
	// create output channel
	dest := make(chan ProcMessage)
	// start a new thread to decode
	go func() {
		defer shutdownWs(ws, mtx)
		defer close(dest)

		for {
			var msg ProcMessage
			err := ws.ReadJSON(&msg)

			// if there is an error, tell everyone to stop
			if err != nil {
				ProcLog.Println(err)
				cancel()
				return
			}

			select {
			// check if anyone else has called cancel()
			case <-ctx.Done():
				ProcLog.Println("stdin cancelling")
				return
			case dest <- msg:
				// do our normal stuff
				ProcLog.Println(msg)
			}
		}
	}()

	return dest
}

// Starts a new goroutine that reads from outgoingMsgChan and sends ProcMessages through sock.
// category is only used for logging, since the outgoing messages already have their own category field
func SendProcConnection(
	ctx context.Context,
	cancel context.CancelFunc,
	ws *websocket.Conn,
	mtx *sync.Mutex,
	outgoingMsgChan chan ProcMessage,
	category string,
) {
	go func() {
		defer shutdownWs(ws, mtx)
		for {
			select {
			case <-ctx.Done():
				ProcLog.Print(category, ": cancelled")
				return
			case msg, ok := <-outgoingMsgChan:
				if ok == false {
					ProcLog.Println("ougoingMsgChan closed")
					return
				}

				mtx.Lock()
				err := ws.WriteJSON(msg)
				mtx.Unlock()

				if err != nil {
					ProcLog.Print(err)
					cancel()
					return
				}
			}
		}
	}()
}

// running & managing the actual instance
// =====================================

// run a lua program
func runLua(ctx context.Context,
	cancel context.CancelFunc,
	sourceDir string,
	stdinChan chan []byte,
	stdoutChan chan ProcMessage,
	stderrChan chan ProcMessage,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	// prepare the process
	proc := exec.CommandContext(ctx, "bin/starter", sourceDir)

	stdin, err := proc.StdinPipe()
	if err != nil {
		ProcLog.Print(err)
		cancel()
		return
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		ProcLog.Print(err)
		cancel()
		return
	}
	stderr, err := proc.StderrPipe()
	if err != nil {
		ProcLog.Print(err)
		cancel()
		return
	}

	err = proc.Start()
	if err != nil {
		ProcLog.Print(err)
		cancel()
		return
	}

	// write to stdin pipe
	go inScanner(ctx, cancel, stdin, stdinChan)
	// read from stdout pipe
	go outScanner(ctx, cancel, stdout, stdoutChan, "stdout")
	// read from stderr pipe
	go outScanner(ctx, cancel, stderr, stderrChan, "stderr")

	err = proc.Wait()
	if err != nil {
		ProcLog.Println(err)
	}
	ProcLog.Println("proc done")
}

// run a new program with CLI I/O being sent over the network
func NewInstance(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	var mtx sync.Mutex

	var wg sync.WaitGroup

	// read the program
	var prog bytes.Buffer
	prog.WriteString("io.stdout:setvbuf(\"no\")\nio.stderr:setvbuf(\"no\")\n")

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
	instancePath, err := os.MkdirTemp("/tmp", "luasource-")
	if err != nil {
		shutdownWs(ws, &mtx)
		ProcLog.Print("failed to make directory for program file:", err)
		return
	}
	defer os.RemoveAll(instancePath)
	err = os.WriteFile(path.Join(instancePath, "main.lua"), prog.Bytes(), os.FileMode(0o600))
	if err != nil {
		shutdownWs(ws, &mtx)
		ProcLog.Print("failed to write program to file:", err)
		return
	}

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
				case "EOF":
					if msg.Body == "stdin" {
						// end stdin, no more input
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
	go runLua(ctx, cancel, instancePath, stdinChan, stdoutChan, stderrChan, &wg)

	wg.Wait()
	ProcLog.Println("program done")
}

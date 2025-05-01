package procweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/quick"

	"github.com/gorilla/websocket"
)

// global values
// ===========================
var httpServerState struct {
	started bool
	srv     *http.Server
	socks   chan *websocket.Conn
}

var upgrader = websocket.Upgrader{}

func startServer() {
	if httpServerState.started == true {
		return
	}
	httpServerState.socks = make(chan *websocket.Conn, 8)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var err error
		listenSock, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			panic(err)
		}
		httpServerState.socks <- listenSock
	})
	var srv http.Handler = mux
	httpServerState.srv = &http.Server{
		Addr:    net.JoinHostPort("localhost", "4041"),
		Handler: srv,
	}

	go func() {
		httpServerState.started = true
		if err := httpServerState.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			httpServerState.started = false
		}
	}()

}

var serverStarted bool = false

// helper functions
// ===========================

// given a nested slice, concatenate all of the items in the input and return the result
func flattenSlice[T any](ts [][]T) []T {
	var result []T
	for _, v := range ts {
		result = append(result, v...)
	}
	return result
}

// create two sockets that are connected to each other
// similar to io.Pipe()
func createSockets() (*websocket.Conn, *websocket.Conn) {

	// start the server, if necessary
	startServer()

	//dialer
	dialSock, _, err := websocket.DefaultDialer.Dial("ws://localhost:4041", nil)
	if err != nil {
		panic(err)
	}

	// listener
	listenSock, ok := <-httpServerState.socks
	if ok == false {
		panic("unable to open listen socket")
	}

	return listenSock, dialSock
}

// given a slice of ProcMessages, concatenate the bodies of
// all the ProcMessages of the given category
func combineCategory(msgs []ProcMessage, category string) ProcMessage {
	var catted strings.Builder
	for _, v := range msgs {
		if v.Category == category {
			catted.WriteString(v.Body)
		}
	}

	return ProcMessage{Category: category, Body: catted.String()}
}

// jsonFromMsg tests
// ===========================

// make sure that the result of ProcMessage->json->ProcMessage is the same as the input
func jsonIdentityHolds(in ProcMessage) bool {
	obj, err := jsonFromMsg(in)
	if err != nil {
		return false
	}
	var out ProcMessage
	err = json.Unmarshal(obj, &out)
	if err != nil {
		return false
	}

	return in == out
}

func TestJsonFromMsg(t *testing.T) {
	c := quick.Config{MaxCount: 10_000}

	if err := quick.Check(jsonIdentityHolds, &c); err != nil {
		t.Error(err)
	}
}

// inScanner tests
// ===========================

// any data we send into inChan should be written to the pipe
// this function checks that the pipe output is equal to the scanner input
func inScannerIdentity(in [][]byte) bool {
	// set up the scanner
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	inChan := make(chan []byte, 8)

	go inScanner(ctx, cancel, writer, inChan)

	// push data into the scanner's channel
	go func() {
		defer close(inChan)
		for _, v := range in {
			inChan <- v
		}
	}()

	result_item := make([]byte, 512)
	var outBuf bytes.Buffer
	for {
		n, err := reader.Read(result_item)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return false
		}
		outBuf.Write(result_item[:n])
	}

	out := outBuf.Bytes()

	flat_in := flattenSlice(in)
	return slices.Equal(flat_in, out)
}

func TestInScanner(t *testing.T) {
	c := quick.Config{MaxCount: 10_000}

	if err := quick.Check(inScannerIdentity, &c); err != nil {
		t.Error(err)
	}
}

// outScanner tests
// ===========================

// any data we send into the outScanner reader should come out of the outScanner channel
// this function checks that the channel output is the same as the pipe input
func outScannerIdentity(in struct {
	Data [][]byte
	Name string
}) bool {
	//set up the scanner
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	outChan := make(chan ProcMessage, 8)

	go outScanner(ctx, cancel, reader, outChan, in.Name)

	// push data into the scanner's pipe
	go func() {
		defer writer.Close()
		for _, v := range in.Data {
			writer.Write(v)
		}
	}()

	// read data from the scanner's channel
	var outBuf bytes.Buffer
	for v := range outChan {
		if v.Category != in.Name {
			return false
		}
		outBuf.WriteString(v.Body)
	}
	out := outBuf.Bytes()

	flat_in := flattenSlice(in.Data)
	return slices.Equal(flat_in, out)
}

func TestOutScanner(t *testing.T) {
	c := quick.Config{MaxCount: 10_000}

	if err := quick.Check(outScannerIdentity, &c); err != nil {
		t.Error(err)
	}
}

// ScanProcConnection tests
// ===========================

// make sure that result of ProcMessage->socket->ProcMessage->channel is the same as the input
func scanConnectionIdentity(in []ProcMessage) bool {
	var mtx sync.Mutex
	readSock, writeSock := createSockets()

	// start the scanner
	ctx, cancel := context.WithCancel(context.Background())
	result_msg_chan := ScanProcConnection(ctx, cancel, readSock, &mtx)

	// send stuff to the scanner
	go func() {
		defer shutdownWs(writeSock, &mtx)
		for _, msg := range in {
			var err error
			mtx.Lock()
			err = writeSock.WriteJSON(msg)
			mtx.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// read from the channel
	out := make([]ProcMessage, 0, len(in))
	for msg := range result_msg_chan {
		out = append(out, msg)
	}

	// compare the slices
	return slices.Equal(in, out)
}

func TestScanProcConnection(t *testing.T) {
	c := quick.Config{MaxCount: 1_000}

	if err := quick.Check(scanConnectionIdentity, &c); err != nil {
		t.Error(err)
	}
}

// SendProcConnection tests
// ===========================

func sendConnectionIdentity(in []ProcMessage) bool {
	var mtx sync.Mutex
	// open the sockets
	readSock, writeSock := createSockets()
	defer shutdownWs(readSock, &mtx)

	// start the sender
	outgoingMsgChan := make(chan ProcMessage, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	SendProcConnection(ctx, cancel, writeSock, &mtx, outgoingMsgChan, "") // the last arg is just for logging

	// put stuff into the sender channel
	go func() {
		defer close(outgoingMsgChan)
		for _, v := range in {
			select {
			case outgoingMsgChan <- v:
			case <-ctx.Done():
				return
			}

		}
	}()

	out := make([]ProcMessage, 0, len(in))
	for {
		var msg ProcMessage
		err := readSock.ReadJSON(&msg)
		if err != nil {
			if websocket.IsCloseError(err, 1000) {
				break
			}
			return false
		}

		out = append(out, msg)
	}

	return slices.Equal(in, out)
}

func TestSendProcConnection(t *testing.T) {
	c := quick.Config{
		MaxCount: 1_000,
	}

	if err := quick.Check(sendConnectionIdentity, &c); err != nil {
		t.Error(err)
	}
}

// NewInstance tests
// we want to make properties for each test program
// probably one for valid inputs and one for invalid inputs
// ===========================

// generators
// -----------------
func msgOfCategory(r *rand.Rand, bodyLenMin int, bodyLenMax int, category string) ProcMessage {
	rangeLen := bodyLenMax - bodyLenMin
	bodyLen := r.Intn(rangeLen) + bodyLenMin

	unicodeMin := 0x0021
	unicodeMax := 0x0096

	var bodyBuilder strings.Builder
	for _ = range bodyLen {
		thisRune := rune(r.Intn(unicodeMax-unicodeMin+1) + unicodeMin)
		bodyBuilder.WriteRune(thisRune)
	}

	return ProcMessage{category, bodyBuilder.String()}
}

func msgCategorySlice(r *rand.Rand, lenMin int, lenMax int, category string) []ProcMessage {
	rangeLen := lenMax - lenMin
	resultLen := r.Intn(rangeLen) + lenMin

	result := make([]ProcMessage, 0, resultLen)
	for _ = range resultLen {
		result = append(result, msgOfCategory(r, 5, 30, category))
	}

	return result
}

// test programs
// -----------------

// make sure that hello.lua produces the correct output
// all is does is print "hi"
func testHelloLua(in []ProcMessage) bool {
	var wg sync.WaitGroup

	f, err := os.Open("test_lua/hello.lua")
	if err != nil {
		return false
	}

	ourSock, instanceSock := createSockets()
	go NewInstance(instanceSock)

	// write in to ourSock
	// this shouldn't do anything in this case because hello.lua doesn't read any input
	wg.Add(1)
	go func() {
		defer wg.Done()

		// program
		for {
			codeBytes := make([]byte, 1024)
			n, err := f.Read(codeBytes)
			if err != nil {
				if errors.Is(io.EOF, err) {
					var codeMsg = ProcMessage{Category: "code", Body: string(codeBytes[:n])}
					err := ourSock.WriteJSON(codeMsg)
					if err != nil {
						panic(err)
					}
					break
				}
				panic(err)
			}

			var codeMsg = ProcMessage{Category: "code", Body: string(codeBytes[:n])}
			err = ourSock.WriteJSON(codeMsg)
			if err != nil {
				panic(err)
			}
		}

		var codeEof = ProcMessage{Category: "EOF", Body: "code"}
		err = ourSock.WriteJSON(codeEof)
		if err != nil {
			panic(err)
		}
		for _, v := range in {
			err := ourSock.WriteJSON(v)
			if err != nil {
				return
			}
		}
	}()

	// read output from ourSock
	var outMsgs []ProcMessage
	for {
		var currentMsg ProcMessage
		err := ourSock.ReadJSON(&currentMsg)
		if err != nil {
			if websocket.IsCloseError(err, 1000) {
				fmt.Println("done reading")
				break
			}
			return false
		}
		outMsgs = append(outMsgs, currentMsg)
	}

	wg.Wait()
	combined_stdout := combineCategory(outMsgs, "stdout")
	combined_stderr := combineCategory(outMsgs, "stderr")
	if combined_stderr.Body != "" {
		return false
	}
	if combined_stdout.Body != "hi\n" {
		return false
	}

	return true
}

func testEchoLua(in []ProcMessage) bool {
	var wg sync.WaitGroup

	// read the program from a file
	f, err := os.Open("test_lua/echo.lua")
	if err != nil {
		return false
	}

	// start the instance
	ourSock, instanceSock := createSockets()
	go NewInstance(instanceSock)

	// write to in sock
	wg.Add(1)
	go func() {
		defer wg.Done()
		// program
		for {
			codeBytes := make([]byte, 1024)
			n, err := f.Read(codeBytes)
			if err != nil {
				if errors.Is(io.EOF, err) {
					var codeMsg = ProcMessage{Category: "code", Body: string(codeBytes[:n])}
					err := ourSock.WriteJSON(codeMsg)
					if err != nil {
						panic(err)
					}
					break
				}
				panic(err)
			}

			var codeMsg = ProcMessage{Category: "code", Body: string(codeBytes[:n])}
			err = ourSock.WriteJSON(codeMsg)
			if err != nil {
				panic(err)
			}
		}

		var codeEof = ProcMessage{Category: "EOF", Body: "code"}
		err = ourSock.WriteJSON(codeEof)
		if err != nil {
			panic(err)
		}

		// stdin
		for _, v := range in {
			err := ourSock.WriteJSON(v)
			if err != nil {
				if websocket.IsCloseError(err, 1000) {
					break
				}
				break
			}
		}
		msg := ProcMessage{"EOF", "stdin"}
		ourSock.WriteJSON(msg)
	}()

	// read the output
	var outMsgs []ProcMessage
	for {
		var currentMsg ProcMessage
		err := ourSock.ReadJSON(&currentMsg)
		fmt.Println(currentMsg)
		if err != nil {
			if websocket.IsCloseError(err, 1000) {
				fmt.Println("done reading")
				break
			}
			fmt.Println("fatal:", err)
			return false
		}

		outMsgs = append(outMsgs, currentMsg)
	}

	wg.Wait()

	// evaluate the output
	combined_stdout := combineCategory(outMsgs, "stdout")
	combined_stderr := combineCategory(outMsgs, "stderr")
	combined_stdin := combineCategory(in, "stdin")
	if combined_stdin.Body[len(combined_stdin.Body)-1] != byte('\n') {
		combined_stdin.Body = combined_stdin.Body + "\n"
	}
	if combined_stdout.Body != combined_stdin.Body {
		fmt.Println("bad stdout:", combined_stdout)
		return false
	}
	if combined_stderr.Body != "" {
		fmt.Println("bad stderr:", combined_stderr)
		return false
	}

	return true
}

// actually run the tests
// -----------------

func TestNewInstance(t *testing.T) {
	c := quick.Config{
		MaxCount: 100,
		Values: func(values []reflect.Value, r *rand.Rand) {
			values[0] = reflect.ValueOf(msgCategorySlice(r, 5, 30, "stdin"))
		},
	}

	if err := quick.Check(testHelloLua, &c); err != nil {
		t.Error(err)
	}

	if err := quick.Check(testEchoLua, &c); err != nil {
		t.Error(err)
	}
}

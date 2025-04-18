package procweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/quick"
)

// ## objectives
// - test different lua programs
// - test different valid inputs
// - test a variety of invalid inputs
// - test a variety of failure conditions
//		- socket errors
//		- lua process errors

// ## what constitutes a correct result?
// - client receives the expected json messages
// - all goroutines shut down when they should
// - there should be no case where any of the goroutines crash (it's ok if the exec process crashes)

// ## what units need to be tested?
// jsonFromMsg (done)
// ScanProcConnection (done)
// SendProcConnection (done)
// inScanner
// outScanner
// runLua (?)

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
func createSockets() (net.Conn, net.Conn) {
	var (
		listen_sock net.Conn
		dial_sock   net.Conn
		addr        string
		wg1         sync.WaitGroup
		wg2         sync.WaitGroup
	)
	// listener
	wg1.Add(1)
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		listener, err := net.Listen("tcp", "")
		if err != nil {
			panic(err)
		}
		addr = listener.Addr().String()
		wg1.Done()
		listen_sock, err = listener.Accept()
		if err != nil {
			panic(err)
		}
	}()

	//dialer
	wg1.Wait()
	dial_sock, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}

	wg2.Wait()
	return listen_sock, dial_sock
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
	c := quick.Config{MaxCount: 1_000}

	if err := quick.Check(inScannerIdentity, &c); err != nil {
		t.Error(err)
	}
}

// We also need to test the whole subsystem (aka NewInstance)

// ScanProcConnection tests
// ===========================

// make sure that result of ProcMessage->json=>socket->ProcMessage=>channel is the same as the input
func scanConnectionIdentity(in []ProcMessage) bool {
	readSock, writeSock := createSockets()

	// start the scanner
	ctx, cancel := context.WithCancel(context.Background())
	result_msg_chan := ScanProcConnection(ctx, cancel, readSock)

	// send stuff to the scanner
	go func() {
		defer writeSock.Close()
		obj := make([]byte, 0, 1024)
		for _, msg := range in {
			var err error
			obj, err = json.Marshal(msg)
			_, err = writeSock.Write(obj)
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
	c := quick.Config{MaxCount: 1000}

	if err := quick.Check(scanConnectionIdentity, &c); err != nil {
		t.Error(err)
	}
}

// SencdProcConnection tests
// ===========================
func sendConnectionIdentity(in struct {
	Messages []ProcMessage
}) bool {
	// open the sockets
	read_sock, write_sock := createSockets()
	defer read_sock.Close()

	// start the sender
	outgoingMsgChan := make(chan ProcMessage, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	SendProcConnection(ctx, cancel, write_sock, outgoingMsgChan, "") // the last arg is just for logging

	// put stuff into the sender channel
	go func() {
		defer close(outgoingMsgChan)
		for _, v := range in.Messages {
			select {
			case outgoingMsgChan <- v:
			case <-ctx.Done():
				return
			}

		}
	}()

	d := json.NewDecoder(read_sock)
	out := make([]ProcMessage, 0, len(in.Messages))
	for {
		var msg ProcMessage
		err := d.Decode(&msg)
		if err != nil {
			if errors.Is(io.EOF, err) {
				break
			}
			return false
		}

		out = append(out, msg)
	}

	return slices.Equal(in.Messages, out)
}

func TestSendProcConnection(t *testing.T) {
	c := quick.Config{MaxCount: 1000}

	if err := quick.Check(sendConnectionIdentity, &c); err != nil {
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
	c := quick.Config{MaxCount: 1_000}

	if err := quick.Check(outScannerIdentity, &c); err != nil {
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

	unicodeMin := 0x0020
	unicodeMax := 0x10FFFF

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

// make sure that hello.lua produces the correct output
// all is does is print "hi"
func testHelloLua(in []ProcMessage) bool {
	var wg sync.WaitGroup

	ourSock, instanceSock := createSockets()
	NewInstance("test_lua/hello.lua", instanceSock)

	// write in to ourSock
	// this shouldn't do anything in this case because hello.lua doesn't read any input
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, v := range in {
			stdinJson, err := jsonFromMsg(v)
			if err != nil {
				return
			}
			_, err = ourSock.Write(stdinJson)
			if err != nil {
				return
			}
		}
	}()

	// read output from ourSock
	var outMsgs []ProcMessage
	d := json.NewDecoder(ourSock)
	for {
		var currentMsg ProcMessage
		err := d.Decode(&currentMsg)
		if err != nil {
			if errors.Is(err, io.EOF) {
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
}

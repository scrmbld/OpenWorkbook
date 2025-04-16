package procweb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
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

// We also need to test the whole subsystem (aka NewInstance)

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

// ScanProcConnection tests
// ===========================

// 1. start ScanProcConnection on a new thread
// 2. send json to its socket
// 3. check the output channel

// make sure that result of ProcMessage->json=>socket->ProcMessage=>channel is the same as the input
func scanConnectionIdentity(in []ProcMessage) bool {
	read_sock, write_sock := createSockets()

	// start the scanner
	ctx, cancel := context.WithCancel(context.Background())
	result_msg_chan := ScanProcConnection(ctx, cancel, read_sock)

	// send stuff to the scanner
	go func() {
		defer write_sock.Close()

		obj := make([]byte, 0, 1024)
		for _, msg := range in {
			var err error
			obj, err = json.Marshal(msg)
			_, err = write_sock.Write(obj)
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
				// fmt.Println(v)
			case <-ctx.Done():
				return
			}

		}
	}()

	d := json.NewDecoder(read_sock)
	out := make([]ProcMessage, 0, len(in.Messages))
	fmt.Println("start reading")
	for {
		var msg ProcMessage
		err := d.Decode(&msg)
		if err != nil {
			if errors.Is(io.EOF, err) {
				break
			}
			return false
		}
		fmt.Println(msg)

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

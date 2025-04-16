package procweb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
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

// given a nested slice, concatenate all of the items in the input and return the result
func flattenSlice[T any](ts [][]T) []T {
	var result []T
	for _, v := range ts {
		result = append(result, v...)
	}
	return result
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
	// fmt.Println(in)
	// fmt.Println(out)
	return slices.Equal(flat_in, out)
}

// func TestInScanner(t *testing.T) {
// 	c := quick.Config{MaxCount: 1_000}
//
// 	if err := quick.Check(inScannerIdentity, &c); err != nil {
// 		t.Error(err)
// 	}
// }

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

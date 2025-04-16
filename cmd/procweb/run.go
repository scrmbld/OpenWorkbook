package procweb

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os/exec"
	"sync"
)

func RunLua(ctx context.Context,
	cancel context.CancelFunc,
	program string,
	stdinChan chan []byte,
	stdoutChan chan ProcMessage,
	stderrChan chan ProcMessage,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	// prepare the process
	proc := exec.CommandContext(ctx, "lua", program)

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

// read from an output pipe
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

// write to stdin
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

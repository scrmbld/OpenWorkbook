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
	proc := exec.Command("lua", program)

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

	// write to stdin
	go func() {
		defer stdin.Close()
		defer ProcLog.Println("closing stdin pipe")

		for {
			select {
			case <-ctx.Done():
				ProcLog.Println("write to stdin cancelled")
				cancel()
				return

			case msg := <-stdinChan:
				ProcLog.Println(string(msg))
				_, err := stdin.Write(msg)
				if err != nil {
					if errors.Is(err, fs.ErrClosed) {
						ProcLog.Println("stdin: closed")
						break
					}
					ProcLog.Fatal("stdin: ", err)
				}
			}
		}
	}()

	// read an output pipe
	outScanner := func(pipe io.ReadCloser, outChan chan ProcMessage, name string) {
		defer pipe.Close()

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

	// read from stdout
	go outScanner(stdout, stdoutChan, "stdout")
	// read from stderr
	go outScanner(stderr, stderrChan, "stderr")

	err = proc.Wait()
	if err != nil {
		ProcLog.Println(err)
	}
	ProcLog.Println("proc done")
}

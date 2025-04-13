package procweb

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os/exec"
	"sync"
)

func RunLua(program string, stdinChan chan []byte, stdoutChan chan []byte, stderrChan chan []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	// prepare the process
	proc := exec.Command("lua", program)

	stdin, err := proc.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := proc.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	err = proc.Start()
	if err != nil {
		log.Fatal(err)
	}

	// write to stdin
	go func() {
		for {
			defer stdin.Close()
			defer fmt.Println("closing stdin pipe")
			var msg []byte
			msg, ok := <-stdinChan
			if ok == false {
				break
			}

			_, err := stdin.Write(msg)
			if err != nil {
				if errors.Is(err, fs.ErrClosed) {
					fmt.Println("stdin: closed")
					break
				}
				log.Fatal("stdin: ", err)
			}
		}
	}()

	// read an output pipe
	outScanner := func(pipe io.ReadCloser, outChan chan []byte, name string) {
		defer pipe.Close()
		defer close(outChan)

		for {
			msg := make([]byte, 2048)
			n, err := pipe.Read(msg)
			if err != nil {
				if errors.Is(err, fs.ErrClosed) {
					fmt.Println(name, ": closed")
					return
				}
				if errors.Is(err, io.EOF) {
					fmt.Println(name, ": EOF")
					return
				}
				log.Fatal(name, ": ", err)
			}

			outChan <- msg[:n]
		}
	}

	// read from stdout
	go outScanner(stdout, stdoutChan, "stdout")
	// read from stderr
	go outScanner(stderr, stderrChan, "stderr")

	err = proc.Wait()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("proc done")
}

package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	"gihub.com/scrmbld/course-site/procrun"
)

// start the lua program
// set up the pipes
// scan the pipes
// wait for messages from the sockets

var wg = sync.WaitGroup{}

func printOutput(out *bufio.Writer, outChan chan []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	defer fmt.Println("done reading")
	for msg := range outChan {
		fmt.Print(string(msg))
		out.Write(msg)
	}
}

func main() {
	stdinChan := make(chan []byte, 8)
	stdoutChan := make(chan []byte, 8)
	stderrChan := make(chan []byte, 8)
	wg.Add(1)
	go procrun.RunLua("lua/test.lua", stdinChan, stdoutChan, stderrChan, &wg)
	stdinScanner := bufio.NewScanner(os.Stdin)
	stdoutWriter := bufio.NewWriter(os.Stdout)
	stderrWriter := bufio.NewWriter(os.Stderr)

	wg.Add(1)
	go func() {
		defer close(stdinChan)
		defer wg.Done()
		defer fmt.Println("done writing")
		for stdinScanner.Scan() {
			text := stdinScanner.Bytes()
			text = append(text, []byte("\n")...)
			fmt.Println(text)
			stdinChan <- text
		}
	}()

	wg.Add(2)
	go printOutput(stdoutWriter, stdoutChan, &wg)
	go printOutput(stderrWriter, stderrChan, &wg)

	wg.Wait()
	fmt.Println("done")
}

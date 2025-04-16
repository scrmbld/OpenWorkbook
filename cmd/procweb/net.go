package procweb

import (
	"context"
	"encoding/json"
	"net"
)

type ProcMessage struct {
	Category string `json:"category"`
	Body     string `json:"body"`
}

func jsonFromMsg(msg ProcMessage) ([]byte, error) {
	result, err := json.Marshal(msg)
	if err != nil {
		return []byte{}, err
	}
	return result, nil
}

// starts a new goroutine that reads from the socket and sends them to the returned channel.
func ScanProcConnection(
	ctx context.Context,
	cancel context.CancelFunc,
	sock net.Conn,
) <-chan ProcMessage {
	// create output channel
	dest := make(chan ProcMessage)
	// start a new thread to decode
	go func() {
		defer sock.Close()
		defer close(dest)
		d := json.NewDecoder(sock)

		for {
			var msg ProcMessage
			err := d.Decode(&msg)

			// if there is an error, tell everyone to stop
			if err != nil {
				ProcLog.Print(err)
				cancel()
				return
			}

			select {
			// check if anyone else has called cancel()
			case <-ctx.Done():
				return
			// do our normal stuff
			case dest <- msg:
				ProcLog.Println(msg)
			}
		}
	}()

	return dest
}

// Starts a new goroutine that consumes outgoingMsgChan and sends ProcMessages through sock.
// category is only used for logging, since the outgoing messages already have their own category field
func SendProcConnection(
	ctx context.Context,
	cancel context.CancelFunc,
	sock net.Conn,
	outgoingMsgChan chan ProcMessage,
	category string,
) {
	go func() {
		defer sock.Close()
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
				encoded, err := jsonFromMsg(msg)
				if err != nil {
					ProcLog.Print(err)
					cancel()
					return
				}
				_, err = sock.Write(encoded)
				if err != nil {
					ProcLog.Print(err)
					cancel()
					return
				}
			}
		}
	}()
}

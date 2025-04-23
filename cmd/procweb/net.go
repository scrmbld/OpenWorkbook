package procweb

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait        = 1 * time.Second
	maxMessageSize   = 8192
	pongWait         = 60 * time.Second
	pingPeriod       = (pongWait * 9) / 10
	closeGracePeriod = 10 * time.Second
)

func shutdownWs(ws *websocket.Conn, mtx *sync.Mutex) {
	mtx.Lock()
	err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	mtx.Unlock()
	if err != nil {
		ProcLog.Print(err)
		ws.Close()
	}
}

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
	ws *websocket.Conn,
	mtx *sync.Mutex,
) <-chan ProcMessage {
	// create output channel
	dest := make(chan ProcMessage)
	// start a new thread to decode
	go func() {
		defer shutdownWs(ws, mtx)
		defer close(dest)

		ws.SetReadLimit(maxMessageSize)
		ws.SetReadDeadline(time.Now().Add(pongWait))
		ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
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

// Starts a new goroutine that consumes outgoingMsgChan and sends ProcMessages through sock.
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
		ws.SetWriteDeadline(time.Now().Add(writeWait))
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

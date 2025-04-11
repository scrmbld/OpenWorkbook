package procnet

import (
	"encoding/json"
	"net"
)

type ProcMessage struct {
	Category string `json:"category"`
	Body     string `json:"body"`
}

func msg_from_json(json_msg []byte) (ProcMessage, error) {
	var result ProcMessage
	err := json.Unmarshal(json_msg, &result)
	if err != nil {
		return ProcMessage{}, err
	}
	return result, nil
}

func json_from_msg(msg ProcMessage) ([]byte, error) {
	result, err := json.Marshal(msg)
	if err != nil {
		return []byte{}, err
	}
	return result, nil
}

func ScanProcConnection(sock net.Conn, incomingMsgChan chan ProcMessage) {
	for {
		d := json.NewDecoder(sock)

		var msg ProcMessage

		err := d.Decode(&msg)
		if err != nil {
			panic(err)
		}

		incomingMsgChan <- msg
	}
}

func SendProcConnection(sock net.Conn, outgoingMsgChan chan ProcMessage) {
	for msg := range outgoingMsgChan {
		encoded, err := json_from_msg(msg)
		if err != nil {
			panic(err)
		}
		_, err = sock.Write(encoded)
		if err != nil {
			panic(err)
		}
	}
}

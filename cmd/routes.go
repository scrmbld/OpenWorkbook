package main

import (
	"log"
	"net/http"

	"gihub.com/scrmbld/OpenWorkbook/views"
	"github.com/a-h/templ"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade: ", err)
		return
	}

	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("write:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func AddRoutes(
	mux *http.ServeMux,
	logger *log.Logger,
) {
	mux.Handle("/index", templ.Handler(views.Index("")))
	mux.Handle("/startcode", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.PostFormValue("code-area")
		log.Println(code)

		http.Redirect(w, r, "/index", http.StatusSeeOther)
	}))

	// static files (at this stage, just images and CSS)
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)
	mux.HandleFunc("/echo", handleRun)
}

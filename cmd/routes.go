package main

import (
	"fmt"
	"log"
	"net/http"

	"gihub.com/scrmbld/OpenWorkbook/cmd/procweb"
	"gihub.com/scrmbld/OpenWorkbook/views/pages"
	"gihub.com/scrmbld/OpenWorkbook/views/pages/love"
	"github.com/a-h/templ"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// create a new instance based on a request to a websocket
func handleRun(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade: ", err)
		return
	}

	procweb.NewInstance(ws)
}

// add all of our routes to the mux in one place
func AddRoutes(
	mux *http.ServeMux,
	logger *log.Logger,
) {
	mux.Handle("/index", templ.Handler(pages.Home()))
	mux.Handle("/courses", templ.Handler(pages.Courses()))
	mux.Handle("/love", templ.Handler(love.LoveHome()))
	for i, v := range love.Chapters {
		mux.Handle(fmt.Sprintf("/love/%d", i), templ.Handler(v))
	}

	// static files
	fs := http.FileServer(http.Dir("./dist"))
	mux.Handle("/", fs)
	mux.HandleFunc("/echo", handleRun)
}

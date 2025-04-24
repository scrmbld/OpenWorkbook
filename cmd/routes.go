package main

import (
	"log"
	"net/http"

	"gihub.com/scrmbld/OpenWorkbook/cmd/procweb"
	"gihub.com/scrmbld/OpenWorkbook/views"
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

	log.Println(w)

	procweb.NewInstance(ws)
}

// add all of our routes to the mux in one place
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

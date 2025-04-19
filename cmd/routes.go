package main

import (
	"log"
	"net/http"

	"gihub.com/scrmbld/course-site/views"
	"github.com/a-h/templ"
)

func AddRoutes(mux *http.ServeMux, logger *log.Logger) {
	mux.Handle("/index", templ.Handler(views.Index("")))
	mux.Handle("/startcode", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.PostFormValue("code-area")
		log.Println(code)

		http.Redirect(w, r, "/index", http.StatusSeeOther)
	}))
}

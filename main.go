package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./static"))

	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("POST /schedule/master", scheduleMasterHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	log.Print("starting server on 4000")

	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}

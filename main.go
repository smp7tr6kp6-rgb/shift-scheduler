package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	log.Print("starting server on 4000")
	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
	})


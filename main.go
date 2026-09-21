package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./static"))

	db, err := openSubmissionStore("data/shift-scheduler.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	submissionStore = db

	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("POST /schedule/master", scheduleMasterHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	log.Print("starting server on 4000")

	err = http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}

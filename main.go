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

	mux.HandleFunc("/", homeHandlerRedirect)

	mux.HandleFunc("GET /login", loginPageHandler)
	mux.HandleFunc("POST /login", loginHandler)
	mux.HandleFunc("/logout", logoutHandler)

	mux.HandleFunc("POST /schedule/master", scheduleMasterHandler)

	mux.HandleFunc("GET /admin", adminHandler)
	mux.HandleFunc("GET /admin/submissions", submissionsHandler)
	mux.HandleFunc("POST /admin/submissions/{id}/approve", approveHandler)
	mux.HandleFunc("POST /admin/submissions/{id}/reject", rejectHandler)

	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	log.Print("starting server on 4000")

	err = http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}

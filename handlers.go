package main

import (
	"fmt"
	"net/http"
	"html/template"
	"log"
)

func home( w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Server", "Go Web Server") 
	
	files := []string{
		"./templates/home.html",
		"./templates/base.html",
		"./templates/nav.html",
		"./templates/schedule.html",
		"./templates/week_view.html",
		"./templates/day_view.html",
	}

	ts, err := templates.ParseFiles(files...)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}

	err = ts.ExecuteTemplate(w, "base", nil)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}	
}
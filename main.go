package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
		fmt.Fprintf(w, "Hello, World!")
	})


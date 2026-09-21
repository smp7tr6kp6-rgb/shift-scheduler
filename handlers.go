package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
)

var templates = template.Must(template.ParseGlob("./templates/*.html"))

func render(w http.ResponseWriter, name string, data any) {
	var page bytes.Buffer
	if err := templates.ExecuteTemplate(&page, name, data); err != nil {
		log.Println(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r, ok := data.(*http.Request); ok && r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write(page.Bytes())
		if err != nil {
			log.Println(err)
		}
		return
	}

	pageData := struct {
		Content template.HTML
	}{
		Content: template.HTML(page.String()),
	}

	if err := templates.ExecuteTemplate(w, "base", pageData); err != nil {
		log.Println(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	render(w, "home", r)
}

func scheduleMasterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span id="id-a82dd6d7-f6de-4617-8432-85bc55ec8091-status" class="status-badge ml-2 flex-shrink-0 inline-block px-2 py-0.5 text-xs font-semibold rounded-full text-yellow-800 bg-yellow-100">Pending</span>`))
}

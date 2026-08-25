package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

type Lang struct {
	Name     string   `json:"name"`
	Snippets []string `json:"snippets"`
}

var langs []Lang
var tmpl *template.Template

func main() {
	b, err := os.ReadFile("data/snippets.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, &langs); err != nil {
		log.Fatal(err)
	}

	tmpl = template.Must(template.ParseFiles("templates/index.html"))

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/", homeHandler)

	log.Println("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, l := range langs {
		if q == "" || strings.Contains(strings.ToLower(l.Name), q) {
			w.Write([]byte(`<button class="btn btn-ghost btn-sm w-full justify-start font-mono" onclick="pick('` + l.Name + `')">` + l.Name + `</button>`))
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.Encode(langs)
	langsJSON := strings.TrimSpace(buf.String())
	langsJSON = strings.ReplaceAll(langsJSON, "</script>", "<\\/script>")
	langsJSON = strings.ReplaceAll(langsJSON, "<!--", "<\\!--")

	data := struct {
		LangsJSON template.JS
	}{
		LangsJSON: template.JS(langsJSON),
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("template execute:", err)
	}
}

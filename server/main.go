package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Lang struct {
	Name        string   `json:"name"`
	Snippets    []string `json:"snippets"`
	TiobeRank   *int     `json:"tiobe_rank,omitempty"`
	TiobeRating *float64 `json:"tiobe_rating,omitempty"`
	GithubRank  *int     `json:"github_rank,omitempty"`
	GithubShare *float64 `json:"github_share,omitempty"`
	ReleaseYear *int     `json:"release_year,omitempty"`
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
	loadPopularity()
	loadReleaseYears()
	{
		cT, cG, cR := 0, 0, 0
		for _, l := range langs {
			if l.TiobeRank != nil { cT++ }
			if l.GithubRank != nil { cG++ }
			if l.ReleaseYear != nil { cR++ }
		}
		log.Printf("popularity: tiobe %d github %d release %d", cT, cG, cR)
	}
	tmpl = template.Must(template.ParseGlob("templates/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("templates/partials/*.html"))

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/", homeHandler)

	log.Println("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func loadPopularity() {
	// tiobe
	if f, err := os.Open("data/tiobe/tiobe_08_2026.csv"); err == nil {
		defer f.Close()
		r := csv.NewReader(f)
		r.TrimLeadingSpace = true
		rows, _ := r.ReadAll()
		m := map[string]int{}
		rm := map[string]float64{}
		for i, row := range rows {
			if i == 0 || len(row) < 3 {
				continue
			}
			n := strings.TrimSpace(row[1])
			rk, _ := strconv.Atoi(strings.TrimSpace(row[0]))
			rat, _ := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
			// handle slash
			for _, part := range strings.Split(n, "/") {
				m[norm(part)] = rk
				rm[norm(part)] = rat
			}
			m[norm(n)] = rk
			rm[norm(n)] = rat
		}
		for i := range langs {
			if v, ok := m[norm(langs[i].Name)]; ok {
				v2 := v
				langs[i].TiobeRank = &v2
				if rat, ok2 := rm[norm(langs[i].Name)]; ok2 {
					r2 := rat
					langs[i].TiobeRating = &r2
				}
			}
		}
	} else {
		log.Println("tiobe load:", err)
	}
	// github
	if f, err := os.Open("data/github/github_language_stats_08_2026.csv"); err == nil {
		defer f.Close()
		r := csv.NewReader(f)
		r.TrimLeadingSpace = true
		rows, _ := r.ReadAll()
		m := map[string]int{}
		sm := map[string]float64{}
		for i, row := range rows {
			if i == 0 || len(row) < 6 {
				continue
			}
			lang := strings.TrimSpace(row[1])
			rk, _ := strconv.Atoi(strings.TrimSpace(row[0]))
			share, _ := strconv.ParseFloat(strings.TrimSpace(row[5]), 64)
			m[norm(lang)] = rk
			sm[norm(lang)] = share
			aliases := strings.Trim(row[3], "\"")
			for _, a := range strings.Split(aliases, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					if _, exists := m[norm(a)]; !exists {
						m[norm(a)] = rk
						sm[norm(a)] = share
					}
				}
			}
		}
		for i := range langs {
			if v, ok := m[norm(langs[i].Name)]; ok {
				v2 := v
				langs[i].GithubRank = &v2
				if s, ok2 := sm[norm(langs[i].Name)]; ok2 {
					s2 := s
					langs[i].GithubShare = &s2
				}
			} else {
				// try alias variations: e.g., "C#" vs "csharp"
				low := norm(langs[i].Name)
				if v, ok := m[low]; ok {
					v2 := v
					langs[i].GithubRank = &v2
				}
			}
		}
	} else {
		log.Println("github load:", err)
	}
}

func loadReleaseYears() {
	b, err := os.ReadFile("data/data.json")
	if err != nil { log.Println("release_year load:", err); return }
	var arr []struct {
		Name        string `json:"name"`
		ReleaseYear *int   `json:"release_year"`
	}
	if err := json.Unmarshal(b, &arr); err != nil { log.Println("release_year parse:", err); return }
	m := map[string]int{}
	for _, e := range arr {
		if e.ReleaseYear != nil {
			m[norm(e.Name)] = *e.ReleaseYear
		}
	}
	for i := range langs {
		if v, ok := m[norm(langs[i].Name)]; ok {
			v2 := v
			langs[i].ReleaseYear = &v2
		}
	}
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

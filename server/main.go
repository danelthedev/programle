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

// ponytail: tolerate "None" strings from gather generate (data.json) -> treat as null
func (l *Lang) UnmarshalJSON(data []byte) error {
	type rawLang map[string]json.RawMessage
	var m rawLang
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["name"]; ok {
		json.Unmarshal(v, &l.Name)
	}
	if v, ok := m["snippets"]; ok {
		json.Unmarshal(v, &l.Snippets)
	}
	l.TiobeRank = parseIntField(m["tiobe_rank"])
	l.TiobeRating = parseFloatField(m["tiobe_rating"])
	l.GithubRank = parseIntField(m["github_rank"])
	if l.GithubRank == nil {
		l.GithubRank = parseIntField(m["github_rank_alt"])
	}
	l.GithubShare = parseFloatField(m["github_share"])
	if l.GithubShare == nil {
		l.GithubShare = parseFloatField(m["github_market_share"])
	}
	l.ReleaseYear = parseIntField(m["release_year"])
	return nil
}

func parseIntField(raw json.RawMessage) *int {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `"None"` {
		return nil
	}
	var i int
	if err := json.Unmarshal(raw, &i); err == nil {
		return &i
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" || s == "None" {
			return nil
		}
		if v, err := strconv.Atoi(s); err == nil {
			return &v
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			v := int(f)
			return &v
		}
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		v := int(f)
		return &v
	}
	return nil
}

func parseFloatField(raw json.RawMessage) *float64 {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `"None"` {
		return nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return &f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" || s == "None" {
			return nil
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return &v
		}
	}
	var i int
	if err := json.Unmarshal(raw, &i); err == nil {
		v := float64(i)
		return &v
	}
	return nil
}

var langs []Lang
var tmpl *template.Template

func main() {
	b, err := os.ReadFile("data/data.json")
	if err != nil {
		log.Fatal(err)
	}
	var all []Lang
	if err := json.Unmarshal(b, &all); err != nil {
		log.Fatal(err)
	}
	// game needs 6 snippets; data.json now single source (133 entries, 73 with 6 snippets)
	langs = nil
	for _, l := range all {
		if len(l.Snippets) == 6 {
			langs = append(langs, l)
		}
	}
	if len(langs) == 0 {
		langs = all
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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

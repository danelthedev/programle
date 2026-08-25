package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runYears(base string) error {
	dataPath := filepath.Join(base, "data/data.json")
	b, err := os.ReadFile(dataPath)
	if err != nil { return err }
	var langs []map[string]any
	json.Unmarshal(b, &langs)
	query := `SELECT ?item ?itemLabel ?year WHERE { ?item wdt:P31/wdt:P279* wd:Q9143; wdt:P571 ?date . BIND(YEAR(?date) AS ?year) SERVICE wikibase:label { bd:serviceParam wikibase:language "en" } }`
	u := "https://query.wikidata.org/sparql?query=" + url.QueryEscape(query) + "&format=json"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "programle/1.0")
	req.Header.Set("Accept", "application/sparql-results+json")
	client := &http.Client{Timeout: 30*time.Second}
	resp, err := client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	bb, _ := io.ReadAll(resp.Body)
	os.MkdirAll(filepath.Join(base,"data"),0755)
	os.WriteFile(filepath.Join(base,"data/wikidata_years.json"), bb, 0644)
	var res struct{
		Results struct{
			Bindings []struct{
				ItemLabel struct{ Value string `json:"value"` } `json:"itemLabel"`
				Year struct{ Value string `json:"value"` } `json:"year"`
			} `json:"bindings"`
		} `json:"results"`
	}
	json.Unmarshal(bb, &res)
	flatMap := map[string]int{}
	for _, b := range res.Results.Bindings {
		y, _ := strconv.Atoi(b.Year.Value)
		if y==0 { continue }
		k := flat(b.ItemLabel.Value)
		if _, ok := flatMap[k]; !ok { flatMap[k]=y }
	}
	overrides := map[string]int{"C sharp":1999,"C plus plus":1985,"Objective-C":1984}
	for k,v := range overrides { flatMap[flat(k)]=v }
	// fallback for bulk-missing (Wikidata label variants)
	fallback := map[string]int{
		"ada":1980, "antlr":1992, "autohotkey":2003, "awk":1977, "basic":1964,
		"batchfile":1981, "beef":2018, "c3":2019, "cmake":2000, "cobol":1959,
		"coldfusion":1995, "dm":1995, "eiffel":1986, "erlang":1986, "fennel":2016,
		"glsl":2004, "gml":1998, "gnuplot":1986, "groovy":2003, "haskell":1990,
		"holyc":2005, "hoon":2013, "idl":1977, "isabelle":1986, "janet":2017,
		"labview":1986, "llvm":2003, "lsl":2003, "odin":2016, "openscad":2010,
		"pascal":1970, "perl":1987, "powershell":2006, "rascal":2009, "rust":2010,
		"scheme":1975, "scilab":1990, "sed":1974, "slang":2017, "sql":1974,
		"transactsql":1989, "vba":1993, "vbscript":1996, "verilog":1984, "vhdl":1980,
		"vimscript":1991, "vimscrip":1991, "assembly":1949,
	}
	for k,v := range fallback {
		if _, ok := flatMap[k]; !ok { flatMap[k]=v }
	}
	// per-language Wikidata fallback for still-missing (live search)
	for _, l := range langs {
		name, _ := l["name"].(string)
		if y, ok := flatMap[flat(name)]; ok {
			l["release_year"]=y; continue
		}
		if y, ok := flatMap[flat(strings.ReplaceAll(name,".",""))]; ok {
			l["release_year"]=y; continue
		}
		if y, ok := fallback[flat(name)]; ok {
			l["release_year"]=y; continue
		}
		// live Wikidata lookup as last resort
		if y := fetchYearFromWikidata(name); y != 0 {
			l["release_year"]=y; flatMap[flat(name)]=y; continue
		}
		l["release_year"]=nil
	}
	j, _ := json.MarshalIndent(langs, "", "  ")
	return os.WriteFile(dataPath, append(j,'\n'), 0644)
}

func fetchYearFromWikidata(name string) int {
	// ponytail: best-effort single lookup, no retry
	client := &http.Client{Timeout: 10*time.Second}
	searchURL := "https://www.wikidata.org/w/api.php?action=wbsearchentities&search=" + url.QueryEscape(name) + "&language=en&format=json&type=item&limit=1"
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "programle/1.0")
	resp, err := client.Do(req)
	if err != nil { return 0 }
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var sRes struct{ Search []struct{ ID string `json:"id"` } `json:"search"` }
	json.Unmarshal(b, &sRes)
	if len(sRes.Search)==0 { return 0 }
	id := sRes.Search[0].ID
	claimsURL := "https://www.wikidata.org/w/api.php?action=wbgetclaims&entity=" + url.QueryEscape(id) + "&property=P571&format=json"
	req2, _ := http.NewRequest("GET", claimsURL, nil)
	req2.Header.Set("User-Agent", "programle/1.0")
	resp2, err := client.Do(req2)
	if err != nil { return 0 }
	defer resp2.Body.Close()
	b2, _ := io.ReadAll(resp2.Body)
	var cRes struct{ Claims map[string][]struct{ Mainsnak struct{ Datavalue struct{ Value struct{ Time string `json:"time"` } `json:"value"` } `json:"datavalue"` } `json:"mainsnak"` } `json:"claims"` }
	json.Unmarshal(b2, &cRes)
	for _, cl := range cRes.Claims["P571"] {
		t := cl.Mainsnak.Datavalue.Value.Time
		if len(t) >= 5 {
			y, _ := strconv.Atoi(t[1:5])
			if y>0 { return y }
		}
	}
	return 0
}

package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ponytail: single pipeline — github → tiobe → generate → snippets

func main() {
	step := flag.String("step", "all", "github|tiobe|generate|years|snippets|all")
	flag.Parse()
	base := findBase()
	need := func(s string) bool { return *step == "all" || *step == s }
	if need("github") {
		fmt.Println("== github ==")
		if err := runGithub(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("tiobe") {
		fmt.Println("== tiobe ==")
		if err := runTiobe(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("generate") {
		fmt.Println("== generate ==")
		if err := runGenerate(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("years") {
		fmt.Println("== years ==")
		if err := runYears(base); err != nil {
			log.Printf("years warn: %v", err)
		}
	}
	if need("snippets") {
		fmt.Println("== snippets ==")
		if err := runSnippets(base); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("pipeline done")
}

func findBase() string {
	_, f, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(f), "..")
	}
	return "."
}

// ---------- github ----------
const linguistURL = "https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml"

func runGithub(base string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN missing")
	}
	langsFile := filepath.Join(base, "data/github/raw/languages.yml")
	resultsFile := filepath.Join(base, fmt.Sprintf("data/github/github_language_stats_%02d_%d.csv", time.Now().Month(), time.Now().Year()))
	// download linguist
	if err := downloadLinguist(langsFile); err != nil {
		log.Printf("linguist dl warn: %v", err)
	}
	langs, err := loadLangs(langsFile)
	if err != nil {
		return err
	}
	results := loadGithubResults(resultsFile)
	skipped := loadSkipped(filepath.Join(base, "data/github/.gather_skip"))
	client := &http.Client{Timeout: 30 * time.Second}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	periodStart := today.AddDate(0, 0, -365)
	i := 0
	for name, info := range langs {
		i++
		if _, ok := results[name]; ok {
			continue
		}
		if skipped[name] {
			continue
		}
		fmt.Printf("[%d/%d] %s\n", i, len(langs), name)
		count, err := githubCount(client, token, name, periodStart, today)
		if err != nil {
			fmt.Printf(" fail %v\n", err)
			continue
		}
		if count < 100 {
			skipped[name] = true
			saveSkipped(filepath.Join(base, "data/github/.gather_skip"), skipped)
			continue
		}
		results[name] = map[string]string{"language": name, "language_id": strconv.Itoa(info.LanguageID), "aliases": strings.Join(info.Aliases, ", "), "active_repositories": strconv.Itoa(count), "market_share": "0"}
		calcGithubScores(results)
		saveGithubResults(resultsFile, results)
		time.Sleep(1800 * time.Millisecond)
	}
	calcGithubScores(results)
	return saveGithubResults(resultsFile, results)
}

type langMeta struct {
	Type       string   `yaml:"type"`
	Color      string   `yaml:"color"`
	Group      string   `yaml:"group"`
	TMScope    string   `yaml:"tm_scope"`
	Generated  bool     `yaml:"generated"`
	LanguageID int      `yaml:"language_id"`
	Aliases    []string `yaml:"aliases"`
	Extensions []string `yaml:"extensions"`
	Filenames  []string `yaml:"filenames"`
}

func downloadLinguist(dst string) error {
	req, _ := http.NewRequest("GET", linguistURL, nil)
	req.Header.Set("User-Agent", "programle")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	os.MkdirAll(filepath.Dir(dst), 0755)
	f, _ := os.Create(dst)
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
func loadLangs(p string) (map[string]langMeta, error) {
	b, _ := os.ReadFile(p)
	raw := map[string]langMeta{}
	yaml.Unmarshal(b, &raw)
	out := map[string]langMeta{}
	for n, d := range raw {
		if d.Type != "programming" || d.Generated || d.TMScope == "none" || d.Group != "" || (len(d.Extensions) == 0 && len(d.Filenames) == 0) || d.Color == "" || d.LanguageID >= 1_100_000_000 {
			continue
		}
		if n == "Aleo" || n == "B" {
			continue
		}
		out[n] = d
	}
	return out, nil
}
func githubCount(client *http.Client, token, lang string, start, end time.Time) (int, error) {
	q := fmt.Sprintf(`language:"%s" pushed:%s..%s`, strings.ReplaceAll(lang, `"`, `\"`), start.Format("2006-01-02"), end.Format("2006-01-02"))
	for attempt := 0; attempt < 6; attempt++ {
		req, _ := http.NewRequest("GET", "https://api.github.com/search/repositories", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "programle")
		qq := req.URL.Query()
		qq.Set("q", q)
		qq.Set("per_page", "1")
		req.URL.RawQuery = qq.Encode()
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}
		if resp.StatusCode == 200 {
			var data map[string]any
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			json.Unmarshal(b, &data)
			if v, ok := data["total_count"].(float64); ok {
				return int(v), nil
			}
			return 0, nil
		}
		resp.Body.Close()
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return 0, fmt.Errorf("max retries")
}
func loadGithubResults(p string) map[string]map[string]string {
	f, err := os.Open(p)
	if err != nil {
		return map[string]map[string]string{}
	}
	defer f.Close()
	r := csv.NewReader(f)
	h, _ := r.Read()
	out := map[string]map[string]string{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		row := map[string]string{}
		for i, k := range h {
			if i < len(rec) {
				row[strings.TrimSpace(k)] = strings.TrimSpace(rec[i])
			}
		}
		if lang := row["language"]; lang != "" {
			out[lang] = row
		}
	}
	return out
}
func saveGithubResults(p string, m map[string]map[string]string) error {
	os.MkdirAll(filepath.Dir(p), 0755)
	f, _ := os.Create(p)
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"rank", "language", "language_id", "aliases", "active_repositories", "market_share"})
	ranked := sortedGithub(m)
	for i, r := range ranked {
		w.Write([]string{strconv.Itoa(i + 1), r["language"], r["language_id"], r["aliases"], r["active_repositories"], r["market_share"]})
	}
	w.Flush()
	return w.Error()
}
func sortedGithub(m map[string]map[string]string) []map[string]string {
	v := []map[string]string{}
	for _, r := range m {
		v = append(v, r)
	}
	sort.Slice(v, func(i, j int) bool {
		a, _ := strconv.Atoi(v[i]["active_repositories"])
		b, _ := strconv.Atoi(v[j]["active_repositories"])
		return a > b
	})
	return v
}
func calcGithubScores(m map[string]map[string]string) {
	total := 0
	for _, r := range m {
		n, _ := strconv.Atoi(r["active_repositories"])
		total += n
	}
	for _, r := range m {
		n, _ := strconv.Atoi(r["active_repositories"])
		share := 0.0
		if total > 0 {
			share = float64(n) / float64(total) * 100
		}
		r["market_share"] = fmt.Sprintf("%.6f", share)
	}
}
func loadSkipped(p string) map[string]bool {
	b, _ := os.ReadFile(p)
	m := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			m[s] = true
		}
	}
	return m
}
func saveSkipped(p string, m map[string]bool) {
	var sb strings.Builder
	for k := range m {
		sb.WriteString(k + "\n")
	}
	os.MkdirAll(filepath.Dir(p), 0755)
	os.WriteFile(p, []byte(sb.String()), 0644)
}

// ---------- tiobe ----------
func runTiobe(base string) error {
	outFile := filepath.Join(base, fmt.Sprintf("data/tiobe/tiobe_%02d_%d.csv", time.Now().Month(), time.Now().Year()))
	req, _ := http.NewRequest("GET", "https://www.tiobe.com/tiobe-index/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	rows := parseTiobe(string(b))
	if len(rows) > 50 {
		rows = rows[:50]
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Rank < rows[j].Rank })
	os.MkdirAll(filepath.Dir(outFile), 0755)
	f, _ := os.Create(outFile)
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"rank", "language", "rating"})
	for _, r := range rows {
		w.Write([]string{strconv.Itoa(r.Rank), r.Language, r.Rating})
	}
	w.Flush()
	return w.Error()
}

type tRow struct{ Rank int; Language, Rating string }

func parseTiobe(s string) []tRow {
	trRe := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	tdRe := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	tagRe := regexp.MustCompile(`<[^>]+>`)
	var out []tRow
	for _, m := range trRe.FindAllStringSubmatch(s, -1) {
		tdsRaw := tdRe.FindAllStringSubmatch(m[1], -1)
		var tds []string
		for _, t := range tdsRaw {
			clean := tagRe.ReplaceAllString(t[1], "")
			clean = html.UnescapeString(strings.TrimSpace(clean))
			tds = append(tds, clean)
		}
		if len(tds) == 0 {
			continue
		}
		rank, err := strconv.Atoi(tds[0])
		if err != nil || rank < 1 || rank > 50 {
			continue
		}
		var lang, rating string
		if len(tds) == 7 {
			lang, rating = tds[4], tds[5]
		} else if len(tds) == 3 {
			lang, rating = tds[1], tds[2]
		} else {
			continue
		}
		if lang == "" {
			continue
		}
		rating = strings.TrimSuffix(strings.TrimSpace(rating), "%")
		out = append(out, tRow{rank, lang, rating})
	}
	return out
}

// ---------- generate ----------
func runGenerate(base string) error {
	now := time.Now()
	gFile := filepath.Join(base, fmt.Sprintf("data/github/github_language_stats_%02d_%d.csv", now.Month(), now.Year()))
	tFile := filepath.Join(base, fmt.Sprintf("data/tiobe/tiobe_%02d_%d.csv", now.Month(), now.Year()))
	outFile := filepath.Join(base, "data/data.json")
	github, aliases := loadGithubForGen(gFile)
	tiobe := loadTiobeForGen(tFile)
	outMap := map[string]*struct {
		Name              string `json:"name"`
		TiobeRank         any    `json:"tiobe_rank"`
		TiobeRating       any    `json:"tiobe_rating"`
		GithubRank        any    `json:"github_rank"`
		GithubMarketShare any    `json:"github_market_share"`
	}{}
	for n, g := range github {
		outMap[n] = &struct {
			Name              string `json:"name"`
			TiobeRank         any    `json:"tiobe_rank"`
			TiobeRating       any    `json:"tiobe_rating"`
			GithubRank        any    `json:"github_rank"`
			GithubMarketShare any    `json:"github_market_share"`
		}{Name: n, TiobeRank: "None", TiobeRating: "None", GithubRank: g.rank, GithubMarketShare: g.share}
	}
	for tName, t := range tiobe {
		if gName := findGithub(tName, github, aliases); gName != "" {
			e := outMap[gName]
			e.TiobeRank = t.rank
			e.TiobeRating = t.rating
		} else {
			outMap[tName] = &struct {
				Name              string `json:"name"`
				TiobeRank         any    `json:"tiobe_rank"`
				TiobeRating       any    `json:"tiobe_rating"`
				GithubRank        any    `json:"github_rank"`
				GithubMarketShare any    `json:"github_market_share"`
			}{Name: tName, TiobeRank: t.rank, TiobeRating: t.rating, GithubRank: "None", GithubMarketShare: "None"}
		}
	}
	var out []any
	for _, e := range outMap {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].(*struct {
		Name              string `json:"name"`
		TiobeRank         any    `json:"tiobe_rank"`
		TiobeRating       any    `json:"tiobe_rating"`
		GithubRank        any    `json:"github_rank"`
		GithubMarketShare any    `json:"github_market_share"`
	}).Name < out[j].(*struct {
		Name              string `json:"name"`
		TiobeRank         any    `json:"tiobe_rank"`
		TiobeRating       any    `json:"tiobe_rating"`
		GithubRank        any    `json:"github_rank"`
		GithubMarketShare any    `json:"github_market_share"`
	}).Name })
	b, _ := json.MarshalIndent(out, "", "  ")
	os.MkdirAll(filepath.Dir(outFile), 0755)
	return os.WriteFile(outFile, append(b, '\n'), 0644)
}

type gInfo struct{ rank int; share string }
type tInfo struct{ rank int; rating string }

func loadGithubForGen(p string) (map[string]gInfo, map[string]string) {
	f, _ := os.Open(p)
	defer f.Close()
	r := csv.NewReader(f)
	hdr, _ := r.Read()
	idxLang, idxRank, idxShare, idxAliases := -1, -1, -1, -1
	for i, h := range hdr {
		h = strings.TrimSpace(h)
		switch h {
		case "language":
			idxLang = i
		case "rank":
			idxRank = i
		case "market_share":
			idxShare = i
		case "aliases":
			idxAliases = i
		}
	}
	m := map[string]gInfo{}
	aliasMap := map[string]string{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		lang := strings.TrimSpace(rec[idxLang])
		var rank int
		fmt.Sscan(rec[idxRank], &rank)
		share := strings.TrimSpace(rec[idxShare])
		m[lang] = gInfo{rank, share}
		if idxAliases >= 0 {
			for _, a := range strings.Split(rec[idxAliases], ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					aliasMap[strings.ToLower(a)] = lang
					aliasMap[normalize(a)] = lang
				}
			}
		}
		aliasMap[strings.ToLower(lang)] = lang
		aliasMap[normalize(lang)] = lang
	}
	return m, aliasMap
}
func loadTiobeForGen(p string) map[string]tInfo {
	f, _ := os.Open(p)
	defer f.Close()
	r := csv.NewReader(f)
	hdr, _ := r.Read()
	for i, h := range hdr {
		hdr[i] = strings.TrimSpace(h)
	}
	idxLang, idxRank, idxRating := -1, -1, -1
	for i, h := range hdr {
		switch h {
		case "language":
			idxLang = i
		case "rank":
			idxRank = i
		case "rating":
			idxRating = i
		}
	}
	m := map[string]tInfo{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		lang := strings.TrimSpace(rec[idxLang])
		var rank int
		fmt.Sscan(rec[idxRank], &rank)
		rating := ""
		if idxRating >= 0 {
			rating = strings.TrimSpace(rec[idxRating])
		}
		m[lang] = tInfo{rank, rating}
	}
	return m
}
func findGithub(tName string, github map[string]gInfo, aliases map[string]string) string {
	if _, ok := github[tName]; ok {
		return tName
	}
	if v, ok := aliases[strings.ToLower(tName)]; ok {
		return v
	}
	for g := range github {
		if strings.EqualFold(g, tName) {
			return g
		}
	}
	norm := normalize(tName)
	if v, ok := aliases[norm]; ok {
		return v
	}
	for g := range github {
		if normalize(g) == norm {
			return g
		}
	}
	if strings.Contains(tName, "/") {
		for _, p := range strings.Split(tName, "/") {
			if g := findGithub(strings.TrimSpace(p), github, aliases); g != "" {
				return g
			}
		}
	}
	trimmed := strings.Trim(strings.ReplaceAll(strings.ReplaceAll(tName, "(", ""), ")", ""), " ")
	if trimmed != tName {
		if g := findGithub(trimmed, github, aliases); g != "" {
			return g
		}
	}
	return ""
}
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "(visual)", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, " language") {
		s = strings.TrimSuffix(s, " language")
	}
	return strings.TrimSpace(s)
}

// ---------- snippets ----------
func runSnippets(base string) error {
	dataPath := filepath.Join(base, "data/data.json")
	filterPath := filepath.Join(base, "data/.rosetta_filter")
	snipPath := filepath.Join(base, "data/snippets.json")
	b, _ := os.ReadFile(dataPath)
	var langs []map[string]any
	json.Unmarshal(b, &langs)
	fb, _ := os.ReadFile(filterPath)
	var filt []string
	json.Unmarshal(fb, &filt)
	filter := map[string]bool{}
	for _, n := range filt {
		filter[n] = true
	}
	rosettaFlat := map[string]string{}
	if rb, err := os.ReadFile("/tmp/rosetta.json"); err == nil {
		var arr []string
		json.Unmarshal(rb, &arr)
		for _, n := range arr {
			rosettaFlat[flat(n)] = n
		}
	} else {
		for _, n := range fetchRosettaNames() {
			rosettaFlat[flat(n)] = n
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	cache := map[string]string{}
	var out []map[string]any
	if ex, err := os.ReadFile(snipPath); err == nil {
		json.Unmarshal(ex, &out)
	}
	done := map[string]bool{}
	for _, l := range out {
		if n, ok := l["name"].(string); ok {
			done[n] = true
		}
	}
	for _, l := range langs {
		name, _ := l["name"].(string)
		if filter[name] || done[name] {
			continue
		}
		rName := findRosetta(name, rosettaFlat)
		if rName == "" {
			rName = name
		}
		members := fetchMembers(client, rName)
		sort.Strings(members)
		if len(members) < 6 {
			filter[name] = true
			filt = append(filt, name)
			sort.Strings(filt)
			j, _ := json.MarshalIndent(filt, "", "  ")
			os.WriteFile(filterPath, append(j, '\n'), 0644)
			fmt.Printf("%s -> %d <6 filter\n", name, len(members))
			continue
		}
		snippets := []string{}
		for _, task := range members {
			if len(snippets) >= 6 { break }
			w, ok := cache[task]
			if !ok {
				w, _ = fetchTask(client, task)
				cache[task] = w
				time.Sleep(350 * time.Millisecond)
			}
			code := extract(w, rName)
			if code == "" { continue }
			snippets = append(snippets, code)
		}
		for len(snippets) < 6 {
			snippets = append(snippets, "")
		}
		// drop if any empty
		hasEmpty := false
		for _, s := range snippets {
			if strings.TrimSpace(s) == "" {
				hasEmpty = true
				break
			}
		}
		if hasEmpty {
			filter[name] = true
			filt = append(filt, name)
			sort.Strings(filt)
			j, _ := json.MarshalIndent(filt, "", "  ")
			os.WriteFile(filterPath, append(j, '\n'), 0644)
			fmt.Printf("%s -> empty snippet filter\n", name)
			continue
		}
		l["snippets"] = snippets
		out = append(out, l)
		fmt.Printf("%s -> 6/6 %v\n", name, members)
		j, _ := json.MarshalIndent(out, "", "  ")
		os.WriteFile(snipPath, append(j, '\n'), 0644)
	}
	// final clean: ensure no empty
	final := []map[string]any{}
	for _, l := range out {
		sn, _ := l["snippets"].([]any)
		if len(sn) != 6 {
			continue
		}
		ok := true
		for _, s := range sn {
			if str, _ := s.(string); strings.TrimSpace(str) == "" {
				ok = false
				break
			}
		}
		if ok {
			final = append(final, l)
		}
	}
	j, _ := json.MarshalIndent(final, "", "  ")
	return os.WriteFile(snipPath, append(j, '\n'), 0644)
}

func fetchRosettaNames() []string {
	names := []string{}
	cont := ""
	for {
		u := "https://rosettacode.org/w/api.php?action=query&list=categorymembers&cmtitle=Category:Programming_Languages&cmtype=subcat&cmlimit=500&format=json"
		if cont != "" {
			u += "&cmcontinue=" + url.QueryEscape(cont)
		}
		resp, _ := http.Get(u)
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var js struct {
			Continue struct{ Cmcontinue string `json:"cmcontinue"` } `json:"continue"`
			Query    struct {
				Members []struct{ Title string `json:"title"` } `json:"categorymembers"`
			} `json:"query"`
		}
		json.Unmarshal(b, &js)
		for _, x := range js.Query.Members {
			names = append(names, strings.TrimPrefix(x.Title, "Category:"))
		}
		if js.Continue.Cmcontinue == "" {
			break
		}
		cont = js.Continue.Cmcontinue
	}
	return names
}


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
	updated:=0
	for _, l := range langs {
		name, _ := l["name"].(string)
		if y, ok := flatMap[flat(name)]; ok {
			l["release_year"]=y; updated++
		} else if y, ok := flatMap[flat(strings.ReplaceAll(name,".",""))]; ok {
			l["release_year"]=y; updated++
		} else {
			l["release_year"]=nil
		}
	}
	j, _ := json.MarshalIndent(langs, "", "  ")
	return os.WriteFile(dataPath, append(j,'\n'), 0644)
}

func flat(s string) string {
	s = strings.ToLower(s)
	if strings.HasSuffix(strings.TrimSpace(s), " language") {
		s = s[:strings.LastIndex(strings.ToLower(s), " language")]
	}
	if strings.HasPrefix(strings.TrimSpace(s), "classic ") {
		s = strings.TrimSpace(s)[8:]
	}
	s = strings.ReplaceAll(s, "#", " sharp ")
	s = strings.ReplaceAll(s, "++", " plus plus ")
	s = strings.ReplaceAll(s, "f***", "fuck")
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return strings.ReplaceAll(s, " ", "")
}
func findRosetta(n string, m map[string]string) string {
	if v, ok := m[flat(n)]; ok {
		return v
	}
	if strings.Contains(n, "/") {
		for _, p := range strings.Split(n, "/") {
			if v, ok := m[flat(p)]; ok {
				return v
			}
		}
	}
	return ""
}
func fetchMembers(client *http.Client, lang string) []string {
	u := "https://rosettacode.org/w/api.php?action=query&list=categorymembers&cmtitle=Category:" + url.PathEscape(lang) + "&cmlimit=500&format=json"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var js struct {
		Query struct {
			Members []struct{ Title string `json:"title"` } `json:"categorymembers"`
		} `json:"query"`
	}
	// ponytail: ignore error, return empty
	_ = json.Unmarshal(b, &js)
	out := []string{}
	for _, m := range js.Query.Members {
		if strings.HasPrefix(m.Title, "Category:") {
			continue
		}
		out = append(out, m.Title)
	}
	return out
}
func fetchTask(client *http.Client, task string) (string, error) {
	u := "https://rosettacode.org/w/api.php?action=parse&page=" + strings.ReplaceAll(task, " ", "_") + "&prop=wikitext&format=json"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var js struct {
		Parse struct{ Wikitext struct{ S string `json:"*"` } `json:"wikitext"` } `json:"parse"`
	}
	json.Unmarshal(b, &js)
	return js.Parse.Wikitext.S, nil
}
func extract(w, lang string) string {
	reH := regexp.MustCompile(`(?i)==\s*\{\{header\|` + regexp.QuoteMeta(lang) + `\}\}\s*==`)
	loc := reH.FindStringIndex(w)
	if loc == nil { return "" }
	sec := w[loc[1]:]
	if nxt := regexp.MustCompile(`(?m)^==\s*\{\{header\|`).FindStringIndex(sec); nxt != nil {
		sec = sec[:nxt[0]]
	}
	reCode := regexp.MustCompile(`(?is)<(?:lang|syntaxhighlight)([^>]*)>(.*?)</(?:lang|syntaxhighlight)>`)
	for _, m := range reCode.FindAllStringSubmatch(sec, -1) {
		attr, code := m[1], strings.TrimSpace(m[2])
		if code == "" { continue }
		if at := regexp.MustCompile(`(?i)lang\s*=\s*"?([^"\s>]+)"?`).FindStringSubmatch(attr); at != nil {
			if flat(at[1]) != flat(lang) { continue }
		}
		if flat(lang) != "shell" && flat(lang) != "bash" && flat(lang) != "batchfile" {
			low := strings.ToLower(code)
			if strings.HasPrefix(strings.TrimSpace(low), "#!/") && (strings.Contains(low, "python") || strings.Contains(low, "perl") || strings.Contains(low, "runhaskell") || strings.Contains(low, "ruby") || strings.Contains(low, "gcc ") || strings.Contains(low, "exec ")) { continue }
			if strings.HasPrefix(strings.TrimSpace(code), "$ ") || strings.HasPrefix(strings.TrimSpace(code), "python -c") { continue }
		}
		return code
	}
	rePre := regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`)
	if m := rePre.FindStringSubmatch(sec); m != nil { return strings.TrimSpace(m[1]) }
	return ""
}

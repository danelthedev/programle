package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const linguistURL = "https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml"

func runGithub(base string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN missing")
	}
	langsFile := filepath.Join(base, "data/github/raw/languages.yml")
	resultsFile := filepath.Join(base, fmt.Sprintf("data/github/github_language_stats_%02d_%d.csv", time.Now().Month(), time.Now().Year()))
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
	// ponytail: quote only when needed, else language:JASS without quotes avoids 93M bug
	esc := strings.ReplaceAll(lang, `"`, `\"`)
	var langPart string
	if strings.ContainsAny(lang, " \t#") || strings.Contains(lang, "++") {
		langPart = fmt.Sprintf(`language:"%s"`, esc)
	} else {
		langPart = fmt.Sprintf(`language:%s`, esc)
	}
	q := fmt.Sprintf(`%s pushed:%s..%s`, langPart, start.Format("2006-01-02"), end.Format("2006-01-02"))
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
				c := int(v)
				// outlier guard: JASS 93M etc -> treat as error, skip
				if c > 80000000 {
					return 0, fmt.Errorf("outlier %d for %s", c, lang)
				}
				return c, nil
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

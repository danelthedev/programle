package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func runWhitelist(base string) error {
	names := fetchRosettaNames()
	sort.Strings(names)
	// batch 50 -> categoryinfo
	whitelist := []string{}
	client := &http.Client{Timeout: 30 * time.Second}
	for i := 0; i < len(names); i += 50 {
		end := i + 50
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]
		titles := ""
		for j, n := range batch {
			if j > 0 {
				titles += "|"
			}
			titles += "Category:" + n
		}
		u := "https://rosettacode.org/w/api.php?action=query&prop=categoryinfo&titles=" + url.QueryEscape(titles) + "&format=json"
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "programle/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var js struct {
			Query struct {
				Pages map[string]struct {
					Title        string `json:"title"`
					CategoryInfo struct {
						Pages int `json:"pages"`
					} `json:"categoryinfo"`
				} `json:"pages"`
			} `json:"query"`
		}
		json.Unmarshal(b, &js)
		for _, p := range js.Query.Pages {
			lang := strings.TrimPrefix(p.Title, "Category:")
			if p.CategoryInfo.Pages >= 6 {
				whitelist = append(whitelist, lang)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	sort.Strings(whitelist)
	outPath := filepath.Join(base, "data/rosetta_whitelist.json")
	os.MkdirAll(filepath.Dir(outPath), 0755)
	j, _ := json.MarshalIndent(whitelist, "", "  ")
	return os.WriteFile(outPath, append(j, '\n'), 0644)
}

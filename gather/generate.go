package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type entry struct {
	Name              string `json:"name"`
	TiobeRank         any    `json:"tiobe_rank"`
	TiobeRating       any    `json:"tiobe_rating"`
	GithubRank        any    `json:"github_rank"`
	GithubMarketShare any    `json:"github_market_share"`
}

type gInfo struct{ rank int; share string }
type tInfo struct{ rank int; rating string }

func runGenerate(base string) error {
	now := time.Now()
	gFile := filepath.Join(base, fmt.Sprintf("data/github/github_language_stats_%02d_%d.csv", now.Month(), now.Year()))
	tFile := filepath.Join(base, fmt.Sprintf("data/tiobe/tiobe_%02d_%d.csv", now.Month(), now.Year()))
	outFile := filepath.Join(base, "data/data.json")
	github := loadGithubForGen(gFile)
	tiobe := loadTiobeForGen(tFile)
	wl := loadWhitelist(base)

	// flat map for merging
	gFlat := map[string]string{}
	for n := range github {
		gFlat[flat(n)] = n
	}

	outMap := map[string]*entry{}
	for n, g := range github {
		if !inWhitelist(n, wl) {
			continue
		}
		outMap[n] = &entry{Name: n, TiobeRank: "None", TiobeRating: "None", GithubRank: g.rank, GithubMarketShare: g.share}
	}
	for tn, t := range tiobe {
		if !inWhitelist(tn, wl) {
			continue
		}
		// direct flat match
		if gn, ok := gFlat[flat(tn)]; ok {
			if e := outMap[gn]; e != nil {
				e.TiobeRank = t.rank
				e.TiobeRating = t.rating
				continue
			}
		}
		// slash split e.g. Delphi/Object Pascal
		if strings.Contains(tn, "/") {
			found := false
			for _, p := range strings.Split(tn, "/") {
				if gn, ok := gFlat[flat(strings.TrimSpace(p))]; ok {
					if e := outMap[gn]; e != nil {
						e.TiobeRank = t.rank
						e.TiobeRating = t.rating
						found = true
						break
					}
				}
			}
			if found {
				continue
			}
		}
		// tiobe-only whitelisted
		outMap[tn] = &entry{Name: tn, TiobeRank: t.rank, TiobeRating: t.rating, GithubRank: "None", GithubMarketShare: "None"}
	}

	out := make([]*entry, 0, len(outMap))
	for _, e := range outMap {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	b, _ := json.MarshalIndent(out, "", "  ")
	os.MkdirAll(filepath.Dir(outFile), 0755)
	return os.WriteFile(outFile, append(b, '\n'), 0644)
}

func loadGithubForGen(p string) map[string]gInfo {
	f, err := os.Open(p)
	if err != nil {
		return map[string]gInfo{}
	}
	defer f.Close()
	r := csv.NewReader(f)
	hdr, _ := r.Read()
	idxLang, idxRank, idxShare := -1, -1, -1
	for i, h := range hdr {
		h = strings.TrimSpace(h)
		switch h {
		case "language":
			idxLang = i
		case "rank":
			idxRank = i
		case "market_share":
			idxShare = i
		}
	}
	m := map[string]gInfo{}
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
	}
	return m
}

func loadTiobeForGen(p string) map[string]tInfo {
	f, err := os.Open(p)
	if err != nil {
		return map[string]tInfo{}
	}
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

func loadWhitelist(base string) map[string]bool {
	b, _ := os.ReadFile(filepath.Join(base, "data/rosetta_whitelist.json"))
	var arr []string
	json.Unmarshal(b, &arr)
	m := map[string]bool{}
	for _, n := range arr {
		m[flat(n)] = true
	}
	return m
}

func inWhitelist(name string, wl map[string]bool) bool {
	if len(wl) == 0 {
		return true
	}
	if wl[flat(name)] {
		return true
	}
	if strings.Contains(name, "/") {
		for _, p := range strings.Split(name, "/") {
			if wl[flat(strings.TrimSpace(p))] {
				return true
			}
		}
	}
	return false
}

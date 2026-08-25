package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

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

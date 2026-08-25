package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func runSnippets(base string) error {
	dataPath := filepath.Join(base, "data/data.json")
	b, _ := os.ReadFile(dataPath)
	var langs []map[string]any
	json.Unmarshal(b, &langs)

	rosettaFlat := map[string]string{}
	for _, n := range fetchRosettaNames() {
		rosettaFlat[flat(n)] = n
	}

	client := &http.Client{Timeout: 30 * time.Second}
	cache := map[string]string{}

	// keep existing snippets if data already has them
	hasSnippets := map[string]bool{}
	for _, l := range langs {
		if sn, ok := l["snippets"]; ok {
			has := false
			switch v := sn.(type) {
			case []any:
				if len(v) >= 6 {
					has = true
					for _, s := range v {
						if str, ok := s.(string); !ok || strings.TrimSpace(str) == "" {
							has = false
							break
						}
					}
				}
			case []string:
				if len(v) >= 6 {
					has = true
					for _, s := range v {
						if strings.TrimSpace(s) == "" {
							has = false
							break
						}
					}
				}
			}
			if has {
				if name, ok := l["name"].(string); ok {
					hasSnippets[name] = true
				}
			}
		}
	}



	for _, l := range langs {
		name, _ := l["name"].(string)
		if hasSnippets[name] {
			continue
		}
		rName := findRosetta(name, rosettaFlat)
		if rName == "" {
			rName = name
		}
		fmt.Printf("fetch %s members...\n", rName)
		members := fetchMembers(client, rName)
		sort.Strings(members)
		if len(members) < 6 {
			fmt.Printf("%s -> %d <6 skip\n", name, len(members))
			continue
		}
		snippets := []string{}
		for _, task := range members {
			if len(snippets) >= 6 {
				break
			}
			w, ok := cache[task]
			if !ok {
				w, _ = fetchTask(client, task)
				cache[task] = w
				time.Sleep(350 * time.Millisecond)
			}
			code := extract(w, rName)
			if code == "" {
				continue
			}
			snippets = append(snippets, code)
		}
		if len(snippets) < 6 {
			fmt.Printf("%s -> %d snippets skip\n", name, len(snippets))
			continue
		}
		l["snippets"] = snippets
		fmt.Printf("%s -> 6/6\n", name)
		j, _ := json.MarshalIndent(langs, "", "  ")
		os.WriteFile(dataPath, append(j, '\n'), 0644)
	}
	// final write (also clean any empty)
	j, _ := json.MarshalIndent(langs, "", "  ")
	if err := os.WriteFile(dataPath, append(j, '\n'), 0644); err != nil {
		return err
	}
	// ponytail: single file now - remove old snippets.json if exists
	os.Remove(filepath.Join(base, "data/snippets.json"))
	return nil
}

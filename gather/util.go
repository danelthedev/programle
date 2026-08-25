package main

import (
	"encoding/json"
	"time"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

func flat(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`\(.*?\)`).ReplaceAllString(s, "")
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

func fetchRosettaNames() []string {
	names := []string{}
	cont := ""
	client := &http.Client{Timeout: 30 * time.Second}
	for {
		u := "https://rosettacode.org/w/api.php?action=query&list=categorymembers&cmtitle=Category:Programming_Languages&cmtype=subcat&cmlimit=500&format=json"
		if cont != "" {
			u += "&cmcontinue=" + url.QueryEscape(cont)
		}
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "programle/1.0")
		resp, err := client.Do(req)
		if err != nil {
			break
		}
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

func fetchMembers(client *http.Client, lang string) []string {
	for attempt := 0; attempt < 3; attempt++ {
		u := "https://rosettacode.org/w/api.php?action=query&list=categorymembers&cmtitle=Category:" + url.PathEscape(lang) + "&cmlimit=500&format=json"
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var js struct {
			Query struct {
				Members []struct{ Title string `json:"title"` } `json:"categorymembers"`
			} `json:"query"`
		}
		_ = json.Unmarshal(b, &js)
		out := []string{}
		for _, m := range js.Query.Members {
			if strings.HasPrefix(m.Title, "Category:") {
				continue
			}
			out = append(out, m.Title)
		}
		if len(out) > 0 || attempt == 2 {
			return out
		}
	}
	return nil
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
	// ponytail: relaxed - accept any code block in section, ignore lang attr
	reCode := regexp.MustCompile(`(?is)<(?:lang|syntaxhighlight)[^>]*>(.*?)</(?:lang|syntaxhighlight)>`)
	for _, m := range reCode.FindAllStringSubmatch(sec, -1) {
		code := strings.TrimSpace(m[1])
		if code == "" { continue }
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

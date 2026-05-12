package websearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

const (
	defaultMaxResults = 5
	requestTimeout    = 10 * time.Second
	duckDuckGoURL     = "https://html.duckduckgo.com/html/"
)

func Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	if searxng := os.Getenv("SEARXNG_URL"); searxng != "" {
		return searchSearxng(ctx, searxng, query, maxResults)
	}
	return searchDuckDuckGo(ctx, query, maxResults)
}

func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]Result, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, "POST", duckDuckGoURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; csghub-lite/1.0)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return parseDuckDuckGoHTML(string(body), maxResults), nil
}

func parseDuckDuckGoHTML(html string, maxResults int) []Result {
	var results []Result
	remaining := html

	for len(results) < maxResults {
		linkStart := indexOfSubstring(remaining, `class="result__a"`)
		if linkStart < 0 {
			break
		}
		remaining = remaining[linkStart:]

		href := extractAttribute(remaining, "href")
		title := extractInnerText(remaining, "</a>")
		remaining = advancePast(remaining, "</a>")

		snippetStart := indexOfSubstring(remaining, `class="result__snippet"`)
		snippet := ""
		if snippetStart >= 0 && snippetStart < 2000 {
			snippetHTML := remaining[snippetStart:]
			snippet = extractInnerText(snippetHTML, "</a>")
			if snippet == "" {
				snippet = extractTagContent(snippetHTML)
			}
		}

		resolvedURL := resolveDDGURL(href)
		if resolvedURL == "" || title == "" {
			continue
		}

		results = append(results, Result{
			Title:   cleanText(title),
			URL:     resolvedURL,
			Snippet: cleanText(snippet),
		})
	}
	return results
}

func resolveDDGURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "//duckduckgo.com/l/?uddg=") || strings.HasPrefix(rawURL, "/l/?uddg=") {
		if idx := strings.Index(rawURL, "uddg="); idx >= 0 {
			encoded := rawURL[idx+5:]
			if ampIdx := strings.Index(encoded, "&"); ampIdx >= 0 {
				encoded = encoded[:ampIdx]
			}
			decoded, err := url.QueryUnescape(encoded)
			if err == nil {
				return decoded
			}
		}
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	return ""
}

func searchSearxng(ctx context.Context, baseURL, query string, maxResults int) ([]Result, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/search")
	if err != nil {
		return nil, fmt.Errorf("invalid SEARXNG_URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("format", "json")
	q.Set("categories", "general")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "csghub-lite/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Searxng request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}

	return parseSearxngJSON(string(body), maxResults), nil
}

func parseSearxngJSON(body string, maxResults int) []Result {
	var results []Result
	remaining := body

	for len(results) < maxResults {
		titleIdx := indexOfSubstring(remaining, `"title"`)
		if titleIdx < 0 {
			break
		}
		remaining = remaining[titleIdx:]

		title := extractJSONStringValue(remaining, "title")
		urlVal := extractJSONStringValue(remaining, "url")
		content := extractJSONStringValue(remaining, "content")

		remaining = advancePast(remaining, `"title"`)

		if urlVal != "" && title != "" {
			results = append(results, Result{
				Title:   title,
				URL:     urlVal,
				Snippet: content,
			})
		}
	}
	return results
}

// Simple string helpers to avoid pulling in an HTML parser dependency.

func indexOfSubstring(s, substr string) int {
	return strings.Index(s, substr)
}

func advancePast(s, marker string) string {
	idx := strings.Index(s, marker)
	if idx < 0 {
		return s[min(len(s), 1):]
	}
	return s[idx+len(marker):]
}

func extractAttribute(s, attr string) string {
	needle := attr + `="`
	idx := strings.Index(s, needle)
	if idx < 0 {
		needle = attr + `='`
		idx = strings.Index(s, needle)
	}
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	quote := s[idx+len(attr)+1]
	end := strings.IndexByte(s[start:], quote)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

func extractInnerText(s, endTag string) string {
	gt := strings.IndexByte(s, '>')
	if gt < 0 {
		return ""
	}
	rest := s[gt+1:]
	endIdx := strings.Index(rest, endTag)
	if endIdx < 0 {
		return rest
	}
	return stripHTMLTags(rest[:endIdx])
}

func extractTagContent(s string) string {
	gt := strings.IndexByte(s, '>')
	if gt < 0 {
		return ""
	}
	rest := s[gt+1:]
	lt := strings.IndexByte(rest, '<')
	if lt < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:lt])
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&#x27;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.TrimSpace(s)
	return s
}

func extractJSONStringValue(s, key string) string {
	needle := `"` + key + `"`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(needle):]
	colonIdx := strings.IndexByte(rest, ':')
	if colonIdx < 0 || colonIdx > 5 {
		return ""
	}
	rest = strings.TrimSpace(rest[colonIdx+1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	var b strings.Builder
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\\' && i+1 < len(rest) {
			i++
			b.WriteByte(rest[i])
			continue
		}
		if rest[i] == '"' {
			return b.String()
		}
		b.WriteByte(rest[i])
	}
	return b.String()
}

package websearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultTimeout    = 5 * time.Second
	defaultMaxResults = 5
	maxResponseBytes  = 2 << 20
	maxSnippetLength  = 600

	ProviderSogou      = "sogou"
	ProviderBaidu      = "baidu"
	ProviderQuark      = "quark"
	ProviderBing       = "bing"
	ProviderDuckDuckGo = "duckduckgo"
)

var ErrNoResults = errors.New("no web search results found")

type Config struct {
	Timeout       time.Duration
	MaxResults    int
	Language      string
	SafeSearch    int
	SafeSearchSet bool
	Providers     []string
	Quick         bool
	HTTPClient    *http.Client

	// ProviderBaseURLs is intended for tests and private deployments that need
	// to point a built-in provider at a controlled compatible endpoint.
	ProviderBaseURLs map[string]string
}

type SearchRequest struct {
	Query string
}

type SearchResponse struct {
	Query    string   `json:"query"`
	Provider string   `json:"provider,omitempty"`
	Results  []Result `json:"results"`
}

type Result struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Snippet     string  `json:"snippet,omitempty"`
	Engine      string  `json:"engine,omitempty"`
	Category    string  `json:"category,omitempty"`
	Score       float64 `json:"score,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
}

type provider interface {
	Name() string
	Search(context.Context, Config, SearchRequest) (SearchResponse, error)
}

type providerResult struct {
	index int
	name  string
	resp  SearchResponse
	err   error
}

func Search(ctx context.Context, cfg Config, req SearchRequest) (SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return SearchResponse{}, fmt.Errorf("query is required")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	providers := providersForConfig(cfg)
	resultsCh := make(chan providerResult, len(providers))
	for i, p := range providers {
		go func(index int, p provider) {
			resp, err := p.Search(ctx, cfg, SearchRequest{Query: query})
			resultsCh <- providerResult{index: index, name: p.Name(), resp: resp, err: err}
		}(i, p)
	}

	maxResults := resolveMaxResults(cfg.MaxResults)
	ordered := make([]providerResult, len(providers))
	received := make([]bool, len(providers))
	for range providers {
		result := <-resultsCh
		ordered[result.index] = result
		received[result.index] = true
		if cfg.Quick {
			if quickResult, normalized, ok := firstQuickProviderResult(ordered, received, query, maxResults); ok {
				return SearchResponse{
					Query:    query,
					Provider: quickResult.name,
					Results:  normalized,
				}, nil
			}
		}
	}

	var errs []string
	var providerNames []string
	resultSets := make([][]Result, 0, len(providers))
	for _, result := range ordered {
		if result.err != nil {
			errs = append(errs, result.name+": "+result.err.Error())
			continue
		}
		normalized := filterResultsByQueryRelevance(query, normalizeResults(result.resp.Results, maxResults))
		if len(normalized) == 0 {
			errs = append(errs, result.name+": no relevant results")
			continue
		}
		providerNames = append(providerNames, result.name)
		resultSets = append(resultSets, normalized)
	}

	merged := mergeProviderResults(resultSets, maxResults)
	if len(merged) > 0 {
		return SearchResponse{
			Query:    query,
			Provider: strings.Join(providerNames, ","),
			Results:  merged,
		}, nil
	}

	if len(errs) == 0 {
		return SearchResponse{}, ErrNoResults
	}
	return SearchResponse{}, fmt.Errorf("web search failed: %s", strings.Join(errs, "; "))
}

func firstQuickProviderResult(ordered []providerResult, received []bool, query string, maxResults int) (providerResult, []Result, bool) {
	for i, result := range ordered {
		if !received[i] {
			return providerResult{}, nil, false
		}
		if result.err != nil {
			continue
		}
		normalized := filterResultsByQueryRelevance(query, normalizeResults(result.resp.Results, maxResults))
		if len(normalized) >= quickSearchResultThreshold(maxResults) {
			return result, normalized, true
		}
	}
	return providerResult{}, nil, false
}

func quickSearchResultThreshold(maxResults int) int {
	if maxResults <= 1 {
		return 1
	}
	if maxResults < 3 {
		return maxResults
	}
	return 3
}

func mergeProviderResults(resultSets [][]Result, maxResults int) []Result {
	if len(resultSets) == 0 {
		return nil
	}
	var candidates []Result
	for row := 0; len(candidates) < maxResults; row++ {
		added := false
		for _, set := range resultSets {
			if row >= len(set) {
				continue
			}
			candidates = append(candidates, set[row])
			added = true
			if len(candidates) >= maxResults*len(resultSets) {
				break
			}
		}
		if !added {
			break
		}
	}
	return normalizeResults(candidates, maxResults)
}

func filterResultsByQueryRelevance(query string, results []Result) []Result {
	terms := relevanceTerms(query)
	if len(terms) == 0 {
		return results
	}
	out := make([]Result, 0, len(results))
	for _, result := range results {
		text := strings.ToLower(result.Title + " " + result.Snippet)
		score := 0
		semanticMatches := 0
		for term, weight := range terms {
			if strings.Contains(text, term) {
				score += weight
				if weight >= 2 {
					semanticMatches++
				}
			}
		}
		if semanticMatches >= minimumSemanticMatches(terms) && score >= 2 {
			out = append(out, result)
		}
	}
	return out
}

func minimumSemanticMatches(terms map[string]int) int {
	count := 0
	for _, weight := range terms {
		if weight >= 2 {
			count++
		}
	}
	if count >= 2 {
		return 2
	}
	return count
}

func relevanceTerms(query string) map[string]int {
	out := map[string]int{}
	var ascii strings.Builder
	var cjk []rune
	flushASCII := func() {
		if ascii.Len() == 0 {
			return
		}
		term := strings.ToLower(ascii.String())
		ascii.Reset()
		if len(term) < 3 || isRelevanceStopTerm(term) {
			return
		}
		if isDigits(term) {
			out[term] = 1
			return
		}
		out[term] = 2
	}
	flushCJK := func() {
		defer func() { cjk = cjk[:0] }()
		if len(cjk) < 2 {
			return
		}
		for i := 0; i < len(cjk)-1; i++ {
			term := string(cjk[i : i+2])
			if isRelevanceStopTerm(term) {
				continue
			}
			out[term] = 2
		}
	}

	for _, r := range query {
		switch {
		case isASCIIAlphaNum(r):
			flushCJK()
			ascii.WriteRune(r)
		case isCJK(r):
			flushASCII()
			cjk = append(cjk, r)
		default:
			flushASCII()
			flushCJK()
		}
	}
	flushASCII()
	flushCJK()
	return out
}

func isASCIIAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isCJK(r rune) bool {
	return (r >= '\u4e00' && r <= '\u9fff') || (r >= '\u3400' && r <= '\u4dbf')
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func isRelevanceStopTerm(term string) bool {
	switch term {
	case "今年", "当前", "最新", "最近", "现在", "今天", "今日", "如何", "什么", "多少", "本赛", "赛季",
		"the", "and", "for", "this", "year", "current", "latest", "recent", "today", "now", "how", "what":
		return true
	default:
		return false
	}
}

func providersForConfig(cfg Config) []provider {
	names := cfg.Providers
	if len(names) == 0 {
		names = defaultProviderOrder(cfg.Language)
	}
	out := make([]provider, 0, len(names))
	seen := map[string]struct{}{}
	for _, raw := range names {
		name := normalizeProviderName(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		switch name {
		case ProviderSogou:
			out = append(out, sogouProvider{})
		case ProviderBaidu:
			out = append(out, baiduProvider{})
		case ProviderQuark:
			out = append(out, quarkProvider{})
		case ProviderBing:
			out = append(out, bingProvider{})
		case ProviderDuckDuckGo:
			out = append(out, duckDuckGoProvider{})
		}
	}
	if len(out) == 0 {
		return []provider{sogouProvider{}, baiduProvider{}, quarkProvider{}, bingProvider{}, duckDuckGoProvider{}}
	}
	return out
}

func defaultProviderOrder(language string) []string {
	lower := strings.ToLower(strings.TrimSpace(language))
	if strings.HasPrefix(lower, "zh") || lower == "" {
		return []string{ProviderSogou, ProviderBaidu, ProviderQuark, ProviderBing, ProviderDuckDuckGo}
	}
	return []string{ProviderBing, ProviderDuckDuckGo, ProviderSogou, ProviderBaidu, ProviderQuark}
}

func normalizeProviderName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "sogou", "sg":
		return ProviderSogou
	case "baidu", "bd":
		return ProviderBaidu
	case "quark", "kk", "shenma", "sm":
		return ProviderQuark
	case "bing", "bi":
		return ProviderBing
	case "duckduckgo", "ddg", "duck":
		return ProviderDuckDuckGo
	default:
		return ""
	}
}

func fetchHTML(ctx context.Context, cfg Config, providerName, rawURL, acceptLanguage string) (*html.Node, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", acceptLanguage)
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 csghub-lite")

	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", providerName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %s", providerName, resp.Status)
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing %s html: %w", providerName, err)
	}
	return doc, nil
}

func providerBaseURL(cfg Config, name, fallback string) string {
	if cfg.ProviderBaseURLs != nil {
		if baseURL := strings.TrimSpace(cfg.ProviderBaseURLs[name]); baseURL != "" {
			return baseURL
		}
	}
	return fallback
}

func searchURL(baseURL, queryParam, query string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	params := u.Query()
	params.Set(queryParam, query)
	u.RawQuery = params.Encode()
	return u.String(), nil
}

func normalizeResults(results []Result, maxResults int) []Result {
	out := make([]Result, 0, maxResults)
	seen := make(map[string]struct{}, maxResults)
	for _, item := range results {
		title := cleanText(item.Title)
		resultURL := strings.TrimSpace(item.URL)
		if title == "" || resultURL == "" || !isHTTPURL(resultURL) {
			continue
		}
		key := strings.ToLower(resultURL)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		item.Title = title
		item.URL = resultURL
		item.Snippet = truncateText(cleanText(item.Snippet), maxSnippetLength)
		item.Engine = cleanText(item.Engine)
		item.Category = cleanText(item.Category)
		item.PublishedAt = cleanText(item.PublishedAt)
		out = append(out, item)
		if len(out) >= maxResults {
			break
		}
	}
	return out
}

func isHTTPURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func resolveMaxResults(maxResults int) int {
	if maxResults <= 0 {
		return defaultMaxResults
	}
	if maxResults > 10 {
		return 10
	}
	return maxResults
}

func cleanText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truncateText(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

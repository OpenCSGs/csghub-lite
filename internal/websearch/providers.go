package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type sogouProvider struct{}

func (sogouProvider) Name() string { return ProviderSogou }

func (p sogouProvider) Search(ctx context.Context, cfg Config, req SearchRequest) (SearchResponse, error) {
	rawURL, err := searchURL(providerBaseURL(cfg, p.Name(), "https://www.sogou.com/web"), "query", req.Query)
	if err != nil {
		return SearchResponse{}, err
	}
	doc, err := fetchHTML(ctx, cfg, p.Name(), rawURL, acceptLanguage(firstNonEmpty(cfg.Language, "zh-CN")))
	if err != nil {
		return SearchResponse{}, err
	}
	return SearchResponse{Query: req.Query, Results: parseSogouResults(doc)}, nil
}

func parseSogouResults(doc *html.Node) []Result {
	nodes := findAll(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && (hasClass(n, "rb") || hasClass(n, "vrwrap"))
	})
	results := make([]Result, 0, len(nodes))
	for _, node := range nodes {
		link := sogouTitleLink(node)
		if link == nil {
			continue
		}
		snippetNode := findFirst(node, hasAnyClass("currentDay", "desc-box", "ft", "attribute-centent", "fz-mid", "space-txt"))
		publishedNode := findFirst(node, func(n *html.Node) bool {
			return isElement("cite")(n) || hasClass(n, "cite-date")
		})
		results = append(results, Result{
			Title:       nodeText(link),
			URL:         sogouResultURL(attr(link, "href"), node),
			Snippet:     nodeText(snippetNode),
			Engine:      ProviderSogou,
			PublishedAt: nodeText(publishedNode),
		})
	}
	return results
}

func sogouTitleLink(root *html.Node) *html.Node {
	title := findFirst(root, func(n *html.Node) bool {
		return isElement("h3")(n) && (hasClass(n, "pt") || hasClass(n, "vr-title"))
	})
	if title == nil {
		return nil
	}
	return firstLink(title)
}

func sogouResultURL(rawURL string, node *html.Node) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "/link?url=") {
		if dataURL := firstAttrUnder(node, "data-url"); isHTTPURL(dataURL) {
			return dataURL
		}
		return "https://www.sogou.com" + rawURL
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return "https://www.sogou.com" + rawURL
	}
	return rawURL
}

type bingProvider struct{}

func (bingProvider) Name() string { return ProviderBing }

func (p bingProvider) Search(ctx context.Context, cfg Config, req SearchRequest) (SearchResponse, error) {
	rawURL, err := searchURL(providerBaseURL(cfg, p.Name(), "https://www.bing.com/search"), "q", req.Query)
	if err != nil {
		return SearchResponse{}, err
	}
	doc, err := fetchHTML(ctx, cfg, p.Name(), rawURL, acceptLanguage(cfg.Language))
	if err != nil {
		return SearchResponse{}, err
	}
	return SearchResponse{Query: req.Query, Results: parseBingResults(doc)}, nil
}

func parseBingResults(doc *html.Node) []Result {
	nodes := findAll(doc, isElementWithClass("li", "b_algo"))
	results := make([]Result, 0, len(nodes))
	for _, node := range nodes {
		link := firstLinkUnder(node, "h2")
		if link == nil {
			continue
		}
		snippetNode := findFirst(node, isElement("p"))
		results = append(results, Result{
			Title:   nodeText(link),
			URL:     attr(link, "href"),
			Snippet: nodeText(snippetNode),
			Engine:  ProviderBing,
		})
	}
	return results
}

type baiduProvider struct{}

func (baiduProvider) Name() string { return ProviderBaidu }

func (p baiduProvider) Search(ctx context.Context, cfg Config, req SearchRequest) (SearchResponse, error) {
	rawURL, err := searchURL(providerBaseURL(cfg, p.Name(), "https://www.baidu.com/s"), "wd", req.Query)
	if err != nil {
		return SearchResponse{}, err
	}
	doc, err := fetchHTML(ctx, cfg, p.Name(), rawURL, acceptLanguage(firstNonEmpty(cfg.Language, "zh-CN")))
	if err != nil {
		return SearchResponse{}, err
	}
	return SearchResponse{Query: req.Query, Results: parseBaiduResults(doc)}, nil
}

func parseBaiduResults(doc *html.Node) []Result {
	nodes := findAll(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && (hasClass(n, "result") || hasClass(n, "result-op"))
	})
	results := make([]Result, 0, len(nodes))
	for _, node := range nodes {
		link := firstLinkUnder(node, "h3")
		if link == nil {
			continue
		}
		snippetNode := findFirst(node, hasAnyClass("c-abstract", "content-right", "c-span-last"))
		results = append(results, Result{
			Title:   nodeText(link),
			URL:     attr(link, "href"),
			Snippet: nodeText(snippetNode),
			Engine:  ProviderBaidu,
		})
	}
	return results
}

type quarkProvider struct{}

func (quarkProvider) Name() string { return ProviderQuark }

func (p quarkProvider) Search(ctx context.Context, cfg Config, req SearchRequest) (SearchResponse, error) {
	rawURL, err := searchURL(providerBaseURL(cfg, p.Name(), "https://quark.sm.cn/s?layout=html&page=1"), "q", req.Query)
	if err != nil {
		return SearchResponse{}, err
	}
	doc, err := fetchHTML(ctx, cfg, p.Name(), rawURL, acceptLanguage(firstNonEmpty(cfg.Language, "zh-CN")))
	if err != nil {
		return SearchResponse{}, err
	}
	return SearchResponse{Query: req.Query, Results: parseQuarkResults(doc)}, nil
}

func parseQuarkResults(doc *html.Node) []Result {
	scripts := findAll(doc, isQuarkDataScript)
	results := make([]Result, 0, len(scripts))
	for _, script := range scripts {
		var payload struct {
			Data struct {
				InitialData map[string]interface{} `json:"initialData"`
			} `json:"data"`
			ExtraData map[string]interface{} `json:"extraData"`
		}
		if err := json.Unmarshal([]byte(scriptText(script)), &payload); err != nil {
			continue
		}
		results = append(results, quarkResultsFromData(stringValue(payload.ExtraData["sc"]), payload.Data.InitialData)...)
	}
	return results
}

func isQuarkDataScript(n *html.Node) bool {
	return isElement("script")(n) &&
		strings.EqualFold(attr(n, "type"), "application/json") &&
		strings.HasPrefix(attr(n, "id"), "s-data-") &&
		attr(n, "data-used-by") == "hydrate"
}

func quarkResultsFromData(category string, data map[string]interface{}) []Result {
	if len(data) == 0 {
		return nil
	}
	switch category {
	case "addition":
		return []Result{quarkResult(
			stringPath(data, "title", "content"),
			stringPath(data, "source", "url"),
			stringPath(data, "summary", "content"),
			"",
		)}
	case "ai_page":
		items := slicePath(data, "list")
		results := make([]Result, 0, len(items))
		for _, item := range items {
			if itemMap := asMap(item); itemMap != nil {
				results = append(results, quarkResult(
					stringPath(itemMap, "title"),
					stringPath(itemMap, "url"),
					quarkContentText(itemMap["content"]),
					formatUnixTimestamp(stringPath(itemMap, "source", "time")),
				))
			}
		}
		return results
	case "nature_result":
		return []Result{quarkResult(
			stringPath(data, "title"),
			stringPath(data, "url"),
			stringPath(data, "desc"),
			"",
		)}
	case "news_uchq":
		items := slicePath(data, "feed")
		results := make([]Result, 0, len(items))
		for _, item := range items {
			if itemMap := asMap(item); itemMap != nil {
				results = append(results, quarkResult(
					stringPath(itemMap, "title"),
					stringPath(itemMap, "url"),
					stringPath(itemMap, "summary"),
					stringPath(itemMap, "time"),
				))
			}
		}
		return results
	case "ss_note":
		return []Result{quarkResult(
			stringPath(data, "title", "content"),
			stringPath(data, "source", "dest_url"),
			stringPath(data, "summary", "content"),
			formatUnixTimestamp(stringPath(data, "source", "time")),
		)}
	case "ss_doc", "ss_kv", "ss_pic", "ss_text", "ss_video", "baike", "structure_web_novel":
		return []Result{quarkStructuredResult(data)}
	default:
		return []Result{quarkStructuredResult(data)}
	}
}

func quarkStructuredResult(data map[string]interface{}) Result {
	return quarkResult(
		firstStringPath(data, []string{"titleProps", "content"}, []string{"title"}),
		firstStringPath(data, []string{"sourceProps", "dest_url"}, []string{"normal_url"}, []string{"url"}),
		firstStringPath(data, []string{"summaryProps", "content"}, []string{"message", "replyContent"}, []string{"show_body"}, []string{"desc"}),
		formatUnixTimestamp(firstStringPath(data, []string{"sourceProps", "time"}, []string{"source", "time"})),
	)
}

func quarkResult(title, rawURL, snippet, publishedAt string) Result {
	return Result{
		Title:       cleanHTMLText(title),
		URL:         rawURL,
		Snippet:     cleanHTMLText(snippet),
		Engine:      ProviderQuark,
		PublishedAt: cleanText(publishedAt),
	}
}

type duckDuckGoProvider struct{}

func (duckDuckGoProvider) Name() string { return ProviderDuckDuckGo }

func (p duckDuckGoProvider) Search(ctx context.Context, cfg Config, req SearchRequest) (SearchResponse, error) {
	rawURL, err := searchURL(providerBaseURL(cfg, p.Name(), "https://html.duckduckgo.com/html/"), "q", req.Query)
	if err != nil {
		return SearchResponse{}, err
	}
	doc, err := fetchHTML(ctx, cfg, p.Name(), rawURL, acceptLanguage(cfg.Language))
	if err != nil {
		return SearchResponse{}, err
	}
	return SearchResponse{Query: req.Query, Results: parseDuckDuckGoResults(doc)}, nil
}

func parseDuckDuckGoResults(doc *html.Node) []Result {
	nodes := findAll(doc, hasAnyClass("result", "web-result"))
	results := make([]Result, 0, len(nodes))
	for _, node := range nodes {
		link := findFirst(node, func(n *html.Node) bool {
			return isElement("a")(n) && (hasClass(n, "result__a") || hasClass(n, "result-link") || strings.TrimSpace(attr(n, "href")) != "")
		})
		if link == nil {
			continue
		}
		snippetNode := findFirst(node, hasAnyClass("result__snippet", "result-snippet"))
		results = append(results, Result{
			Title:   nodeText(link),
			URL:     duckDuckGoResultURL(attr(link, "href")),
			Snippet: nodeText(snippetNode),
			Engine:  ProviderDuckDuckGo,
		})
	}
	return results
}

func acceptLanguage(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return "zh-CN,zh;q=0.9,en;q=0.8"
	}
	return fmt.Sprintf("%s,%s;q=0.9,en;q=0.8", language, strings.Split(language, "-")[0])
}

func firstAttrUnder(root *html.Node, key string) string {
	node := findFirst(root, func(n *html.Node) bool {
		return strings.TrimSpace(attr(n, key)) != ""
	})
	return attr(node, key)
}

func scriptText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

func cleanHTMLText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	doc, err := html.Parse(bytes.NewBufferString(raw))
	if err != nil {
		return cleanText(raw)
	}
	return nodeText(doc)
}

func stringPath(data map[string]interface{}, path ...string) string {
	var current interface{} = data
	for _, key := range path {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = currentMap[key]
	}
	return stringValue(current)
}

func firstStringPath(data map[string]interface{}, paths ...[]string) string {
	for _, path := range paths {
		if value := stringPath(data, path...); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func slicePath(data map[string]interface{}, path ...string) []interface{} {
	var current interface{} = data
	for _, key := range path {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = currentMap[key]
	}
	items, _ := current.([]interface{})
	return items
}

func asMap(value interface{}) map[string]interface{} {
	out, _ := value.(map[string]interface{})
	return out
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func quarkContentText(value interface{}) string {
	switch v := value.(type) {
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := stringValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " | ")
	default:
		return stringValue(value)
	}
}

func formatUnixTimestamp(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return ""
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return raw
	}
	return time.Unix(seconds, 0).Format("2006-01-02")
}

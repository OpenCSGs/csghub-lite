package websearch

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

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

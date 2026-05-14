package websearch

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func findAll(root *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func findFirst(root *html.Node, match func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if match(root) {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if got := findFirst(c, match); got != nil {
			return got
		}
	}
	return nil
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	if n != nil {
		walk(n)
	}
	return cleanText(b.String())
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func isElement(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		return n.Type == html.ElementNode && strings.EqualFold(n.Data, tag)
	}
}

func hasClass(n *html.Node, class string) bool {
	class = strings.TrimSpace(class)
	if class == "" {
		return false
	}
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == class {
			return true
		}
	}
	return false
}

func hasAnyClass(classes ...string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		for _, class := range classes {
			if hasClass(n, class) {
				return true
			}
		}
		return false
	}
}

func isElementWithClass(tag, class string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		return isElement(tag)(n) && hasClass(n, class)
	}
}

func firstLink(root *html.Node) *html.Node {
	return findFirst(root, func(n *html.Node) bool {
		return isElement("a")(n) && strings.TrimSpace(attr(n, "href")) != ""
	})
}

func firstLinkUnder(root *html.Node, tag string) *html.Node {
	section := findFirst(root, isElement(tag))
	if section == nil {
		return firstLink(root)
	}
	return firstLink(section)
}

func duckDuckGoResultURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	if u.Path == "/l/" {
		if uddg := u.Query().Get("uddg"); uddg != "" {
			return uddg
		}
	}
	return rawURL
}

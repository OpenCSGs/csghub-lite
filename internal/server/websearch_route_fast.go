package server

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	webSearchGreetingPattern = regexp.MustCompile(`(?i)^(你好|您好|嗨|在吗|hi|hello|hey|thanks|thank you|thx|ok|okay|好的|谢谢|再见|bye|morning|evening)([!?.。！？~\s]*)$`)
	webSearchCodeOnlyPattern = regexp.MustCompile("(?s)^```")
)

func tryFastWebSearchRoute(userQuery string, now time.Time) (webSearchRoute, bool) {
	q := strings.TrimSpace(userQuery)
	if q == "" {
		return webSearchRoute{}, false
	}
	lower := strings.ToLower(q)

	if webSearchGreetingPattern.MatchString(q) {
		return webSearchRoute{
			Action:     webSearchActionSkip,
			Reason:     fastWebSearchSkipReason(q),
			Confidence: 0.97,
		}, true
	}

	if webSearchCodeOnlyPattern.MatchString(q) && !hasExplicitWebSearchIntent(q, lower) {
		return webSearchRoute{
			Action:     webSearchActionSkip,
			Reason:     fastWebSearchSkipReason(q),
			Confidence: 0.9,
		}, true
	}

	if hasExplicitWebSearchIntent(q, lower) || hasLikelyFreshFactsIntent(q, lower) {
		searchQuery := enrichWebSearchQuery(compactWebSearchQuery(q), now)
		return webSearchRoute{
			Action:     webSearchActionSearch,
			Query:      searchQuery,
			Reason:     fastWebSearchSearchReason(q),
			Confidence: 0.93,
		}, true
	}

	if looksLikeFollowUpAnswerableLocally(q, lower) {
		return webSearchRoute{
			Action:     webSearchActionSkip,
			Reason:     fastWebSearchSkipReason(q),
			Confidence: 0.88,
		}, true
	}

	return webSearchRoute{}, false
}

func hasExplicitWebSearchIntent(_, lower string) bool {
	markers := []string{
		"搜索", "搜一下", "搜下", "查一下", "查查", "帮我查", "帮我搜", "联网", "网上搜", "网上查", "检索",
		"search for", "look up", "web search", "google ", "bing ",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func hasLikelyFreshFactsIntent(q, lower string) bool {
	if hasRelativeTimeTerm(q) {
		return true
	}
	markers := []string{
		"天气", "气温", "weather", "新闻", "news", "股价", "股票", "汇率", "比分", "赛程", "战绩",
		"发布了什么", "最新版本", "多少钱", "价格", "票房", "热搜",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func looksLikeFollowUpAnswerableLocally(q, lower string) bool {
	if utf8.RuneCountInString(q) > 40 {
		return false
	}
	markers := []string{
		"继续", "再说", "展开", "详细点", "总结一下", "换个说法", "翻译成", "translate", "rewrite", "continue",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func compactWebSearchQuery(query string) string {
	query = strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if query == "" {
		return query
	}
	if utf8.RuneCountInString(query) <= 80 {
		return query
	}
	runes := []rune(query)
	return string(runes[:80])
}

func fastWebSearchSkipReason(query string) string {
	if isLikelyChineseText(query) {
		return "无需联网即可回答"
	}
	return "no web search needed"
}

func fastWebSearchSearchReason(query string) string {
	if isLikelyChineseText(query) {
		return "问题需要最新或具体事实，快速路由到搜索"
	}
	return "likely needs fresh facts from the web"
}

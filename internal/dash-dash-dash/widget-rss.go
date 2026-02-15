package dashdashdash

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
)

var (
	rssWidgetTemplate                 = mustParseTemplate("rss-list.html", "widget-base.html")
	rssWidgetDetailedListTemplate     = mustParseTemplate("rss-detailed-list.html", "widget-base.html")
	rssWidgetHorizontalCardsTemplate  = mustParseTemplate("rss-horizontal-cards.html", "widget-base.html")
	rssWidgetHorizontalCards2Template = mustParseTemplate("rss-horizontal-cards-2.html", "widget-base.html")
)

var feedParser = gofeed.NewParser()

type rssWidget struct {
	widgetBase       `yaml:",inline"`
	FeedRequests     []rssFeedRequest `yaml:"feeds"`
	Style            string           `yaml:"style"`
	ThumbnailHeight  float64          `yaml:"thumbnail-height"`
	CardHeight       float64          `yaml:"card-height"`
	Limit            int              `yaml:"limit"`
	CollapseAfter    int              `yaml:"collapse-after"`
	SingleLineTitles bool             `yaml:"single-line-titles"`
	PreserveOrder    bool             `yaml:"preserve-order"`

	Items          rssFeedItemList `yaml:"-"`
	NoItemsMessage string          `yaml:"-"`

	cachedFeedsMutex sync.Mutex
	cachedFeeds      map[string]*cachedRSSFeed `yaml:"-"`
}
func (widget *rssWidget) IsRefreshable() bool {
	return true
}
func (widget *rssWidget) initialize() error {
	widget.withTitle("RSS Feed").withCacheDuration(2 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 25
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.ThumbnailHeight < 0 {
		widget.ThumbnailHeight = 0
	}

	if widget.CardHeight < 0 {
		widget.CardHeight = 0
	}

	if widget.Style == "detailed-list" {
		for i := range widget.FeedRequests {
			widget.FeedRequests[i].IsDetailed = true
		}
	}

	widget.NoItemsMessage = "No items were returned from the feeds."
	widget.cachedFeeds = make(map[string]*cachedRSSFeed)

	return nil
}

// needsImages returns true if the widget style displays images
func (widget *rssWidget) needsImages() bool {
	// Only extract images for styles that actually display them
	return widget.Style == "horizontal-cards" || 
	       widget.Style == "horizontal-cards-2" || 
	       widget.Style == "detailed-list"
}

func (widget *rssWidget) update(ctx context.Context) {
	items, err := widget.fetchItemsFromFeeds(ctx)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if !widget.PreserveOrder {
		items.sortByNewest()
	}

	if len(items) > widget.Limit {
		items = items[:widget.Limit]
	}

	widget.Items = items
}

func (widget *rssWidget) Render() template.HTML {
	if widget.Style == "horizontal-cards" {
		return widget.renderTemplate(widget, rssWidgetHorizontalCardsTemplate)
	}

	if widget.Style == "horizontal-cards-2" {
		return widget.renderTemplate(widget, rssWidgetHorizontalCards2Template)
	}

	if widget.Style == "detailed-list" {
		return widget.renderTemplate(widget, rssWidgetDetailedListTemplate)
	}

	// "list" and "vertical-list" (alias) both use the list template
	return widget.renderTemplate(widget, rssWidgetTemplate)
}

type cachedRSSFeed struct {
	etag         string
	lastModified string
	items        []rssFeedItem
}

type rssFeedItem struct {
	ChannelName string
	ChannelURL  string
	Title       string
	Link        string
	ImageURL    string
	Categories  []string
	Description string
	PublishedAt time.Time
}

type rssFeedRequest struct {
	URL             string            `yaml:"url"`
	Title           string            `yaml:"title"`
	HideCategories  bool              `yaml:"hide-categories"`
	HideDescription bool              `yaml:"hide-description"`
	Limit           int               `yaml:"limit"`
	ItemLinkPrefix  string            `yaml:"item-link-prefix"`
	Headers         map[string]string `yaml:"headers"`
	IsDetailed      bool              `yaml:"-"`
}

type rssFeedItemList []rssFeedItem

func (f rssFeedItemList) sortByNewest() rssFeedItemList {
	sort.Slice(f, func(i, j int) bool {
		return f[i].PublishedAt.After(f[j].PublishedAt)
	})

	return f
}

func (widget *rssWidget) fetchItemsFromFeeds(ctx context.Context) (rssFeedItemList, error) {
	requests := widget.FeedRequests

	job := newJob(func(req rssFeedRequest) ([]rssFeedItem, error) {
		return widget.fetchItemsFromFeedTask(ctx, req)
	}, requests).withWorkers(30)
	feeds, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	failed := 0
	entries := make(rssFeedItemList, 0, len(feeds)*10)
	seen := make(map[string]struct{})

	for i := range feeds {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to get RSS feed", "url", requests[i].URL, "error", errs[i])
			continue
		}

		for _, item := range feeds[i] {
			if _, exists := seen[item.Link]; exists {
				continue
			}
			entries = append(entries, item)
			seen[item.Link] = struct{}{}
		}
	}

	// When all feeds fail, return errNoContent so we do not cache a successful result; the next update will retry.
	if failed == len(requests) {
		return nil, errNoContent
	}

	if failed > 0 {
		return entries, fmt.Errorf("%w: missing %d RSS feeds", errPartialContent, failed)
	}

	return entries, nil
}

func (widget *rssWidget) fetchItemsFromFeedTask(ctx context.Context, request rssFeedRequest) ([]rssFeedItem, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", request.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", userAgentString)

	widget.cachedFeedsMutex.Lock()
	cache, isCached := widget.cachedFeeds[request.URL]
	if isCached {
		if cache.etag != "" {
			req.Header.Add("If-None-Match", cache.etag)
		}
		if cache.lastModified != "" {
			req.Header.Add("If-Modified-Since", cache.lastModified)
		}
	}
	widget.cachedFeedsMutex.Unlock()

	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && isCached {
		return cache.items, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, request.URL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}

	feed, err := feedParser.ParseString(string(body))
	if err != nil {
		return nil, err
	}

	// Store ETag and Last-Modified headers for next request
	etag := resp.Header.Get("ETag")
	lastModified := resp.Header.Get("Last-Modified")

	if request.Limit > 0 && len(feed.Items) > request.Limit {
		feed.Items = feed.Items[:request.Limit]
	}

	items := make(rssFeedItemList, 0, len(feed.Items))

	for i := range feed.Items {
		item := feed.Items[i]

		rssItem := rssFeedItem{
			ChannelURL: feed.Link,
		}

		if request.ItemLinkPrefix != "" {
			rssItem.Link = request.ItemLinkPrefix + item.Link
		} else if strings.HasPrefix(item.Link, "http://") || strings.HasPrefix(item.Link, "https://") {
			rssItem.Link = item.Link
		} else {
			parsedUrl, err := url.Parse(feed.Link)
			if err != nil {
				parsedUrl, err = url.Parse(request.URL)
			}

			if err == nil {
				var link string

				if len(item.Link) > 0 && item.Link[0] == '/' {
					link = item.Link
				} else {
					link = "/" + item.Link
				}

				// Use strings.Builder for efficient concatenation
				var urlBuilder strings.Builder
				urlBuilder.WriteString(parsedUrl.Scheme)
				urlBuilder.WriteString("://")
				urlBuilder.WriteString(parsedUrl.Host)
				urlBuilder.WriteString(link)
				rssItem.Link = urlBuilder.String()
			}
		}

		if item.Title != "" {
			rssItem.Title = html.UnescapeString(item.Title)
		} else {
			rssItem.Title = shortenFeedDescriptionLen(item.Description, 100)
		}

		if request.IsDetailed {
			if !request.HideDescription && item.Description != "" && item.Title != "" {
				rssItem.Description = shortenFeedDescriptionLen(item.Description, 200)
			}

			if !request.HideCategories {
				var categories = make([]string, 0, 6)

				for _, category := range item.Categories {
					if len(categories) == 6 {
						break
					}

					if len(category) == 0 || len(category) > 30 {
						continue
					}

					categories = append(categories, category)
				}

				rssItem.Categories = categories
			}
		}

		if request.Title != "" {
			rssItem.ChannelName = request.Title
		} else {
			rssItem.ChannelName = feed.Title
		}

		// Only extract images if the widget style displays them (performance optimization)
		if widget.needsImages() {
			if item.Image != nil {
				rssItem.ImageURL = item.Image.URL
			} else if thumbURL := findThumbnailInItemExtensions(item); thumbURL != "" {
				rssItem.ImageURL = thumbURL
			} else if itemHTML := getItemHTML(item, feed.Link, request.URL); itemHTML != "" {
				baseURL := feed.Link
				if baseURL == "" {
					baseURL = request.URL
				}
				if parsed, err := url.Parse(baseURL); err == nil && parsed.Host != "" {
					baseURL = parsed.Scheme + "://" + parsed.Host
				}
				if imgURL := firstImageFromHTML(itemHTML, baseURL); imgURL != "" {
					rssItem.ImageURL = imgURL
				}
			}
			if rssItem.ImageURL == "" && feed.Image != nil {
				feedImageURL := feed.Image.URL
				if len(feedImageURL) > 0 && feedImageURL[0] == '/' {
					feedImageURL = strings.TrimRight(feed.Link, "/") + feedImageURL
				}
				if feedImageURL != "" && !looksLikeFavicon(feedImageURL) {
					rssItem.ImageURL = feedImageURL
				}
			}
		}

		if item.PublishedParsed != nil {
			rssItem.PublishedAt = *item.PublishedParsed
		} else {
			rssItem.PublishedAt = time.Now()
		}

		items = append(items, rssItem)
	}

// Update cache with ETag and Last-Modified for future conditional requests
if etag != "" || lastModified != "" {
	widget.cachedFeedsMutex.Lock()
	widget.cachedFeeds[request.URL] = &cachedRSSFeed{
		etag:         etag,
		lastModified: lastModified,
		items:        items,
	}
	widget.cachedFeedsMutex.Unlock()
}

return items, nil
}

func findThumbnailInItemExtensions(item *gofeed.Item) string {
	media, ok := item.Extensions["media"]

	if !ok {
		return ""
	}

	return recursiveFindThumbnailInExtensions(media)
}

// firstImageFromHTML extracts the first <img src="..."> from HTML and resolves relative URLs against baseURL.
func firstImageFromHTML(htmlContent, baseURL string) string {
	if htmlContent == "" || strings.TrimSpace(htmlContent) == "" {
		return ""
	}
	matches := firstImgSrcPattern.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		return ""
	}
	src := html.UnescapeString(strings.TrimSpace(matches[1]))
	if src == "" {
		return ""
	}
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return src
	}
	ref, err := url.Parse(src)
	if err != nil {
		return src
	}
	return base.ResolveReference(ref).String()
}

// looksLikeFavicon returns true if the URL is likely a favicon/site icon rather than a post thumbnail.
func looksLikeFavicon(imgURL string) bool {
	if imgURL == "" {
		return true
	}
	ul := strings.ToLower(imgURL)
	if strings.Contains(ul, "favicon") {
		return true
	}
	if strings.Contains(ul, "icon") && (strings.Contains(ul, "apple") || strings.Contains(ul, "32") || strings.Contains(ul, "16")) {
		return true
	}
	if faviconSizeInURLPattern.MatchString(ul) {
		return true
	}
	return false
}

// getItemHTML returns the best available HTML content from a feed item for image extraction.
func getItemHTML(item *gofeed.Item, feedLink, fallbackURL string) string {
	base := feedLink
	if base == "" {
		base = fallbackURL
	}
	if item.Content != "" {
		return item.Content
	}
	if item.Description != "" {
		return item.Description
	}
	return ""
}

func recursiveFindThumbnailInExtensions(extensions map[string][]gofeedext.Extension) string {
	for _, exts := range extensions {
		for _, ext := range exts {
			if ext.Name == "thumbnail" || ext.Name == "image" {
				if url, ok := ext.Attrs["url"]; ok {
					return url
				}
			}

			if ext.Children != nil {
				if url := recursiveFindThumbnailInExtensions(ext.Children); url != "" {
					return url
				}
			}
		}
	}

	return ""
}

var htmlTagsWithAttributesPattern = regexp.MustCompile(`<\/?[a-zA-Z0-9-]+ *(?:[a-zA-Z-]+=(?:"|').*?(?:"|') ?)* *\/?>`)
var firstImgSrcPattern = regexp.MustCompile(`(?i)<img[^>]+?src=["']([^"']+)["']`)
var faviconSizeInURLPattern = regexp.MustCompile(`[?&][sw]=1[0-6](?:&|$)`)

func sanitizeFeedDescription(description string) string {
	if description == "" {
		return ""
	}

	description = strings.ReplaceAll(description, "\n", " ")
	description = htmlTagsWithAttributesPattern.ReplaceAllString(description, "")
	description = sequentialWhitespacePattern.ReplaceAllString(description, " ")
	description = strings.TrimSpace(description)
	description = html.UnescapeString(description)

	return description
}

func shortenFeedDescriptionLen(description string, maxLen int) string {
	description, _ = limitStringLength(description, 1000)
	description = sanitizeFeedDescription(description)
	description, limited := limitStringLength(description, maxLen)

	if limited {
		description += "…"
	}

	return description
}

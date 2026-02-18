package dashdashdash

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var scraperWidgetTemplate = mustParseTemplate("scraper.html", "widget-base.html")

type scraperWidget struct {
	widgetBase       `yaml:",inline"`
	Items            []scraperItem `yaml:"items"`
	SingleLine       bool          `yaml:"single-line"`
	ShowItemTitles   bool          `yaml:"show-item-titles"`
	ScrapedData      []scraperResult `yaml:"-"`
}

type scraperItem struct {
	Title     string            `yaml:"title"`
	URL       string            `yaml:"url"`
	Selectors []scraperSelector `yaml:"selectors"`
}

type scraperSelector struct {
	Selector string `yaml:"selector"`
	Prefix   string `yaml:"prefix"`
	Suffix   string `yaml:"suffix"`
	Attr     string `yaml:"attr"` // Optional: extract attribute instead of text (e.g., "href", "src")
}

type scraperResult struct {
	Title  string
	URL    string
	Values []string
	Error  string
	SingleLine bool
}

func (widget *scraperWidget) IsRefreshable() bool {
	return true
}

func (widget *scraperWidget) initialize() error {
	widget.withTitle("Scraper").withCacheDuration(30 * time.Minute)

	if len(widget.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}

	// Set default to show item titles if there are multiple items
	if len(widget.Items) > 1 {
		widget.ShowItemTitles = true
	}

	// Validate items
	for i, item := range widget.Items {
		if item.URL == "" {
			return fmt.Errorf("item %d: URL is required", i+1)
		}
		if len(item.Selectors) == 0 {
			return fmt.Errorf("item %d: at least one selector is required", i+1)
		}
	}

	return nil
}

func (widget *scraperWidget) update(ctx context.Context) {
	results, err := widget.scrapeItems(ctx)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.ScrapedData = results
}

func (widget *scraperWidget) Render() template.HTML {
	return widget.renderTemplate(widget, scraperWidgetTemplate)
}

func (widget *scraperWidget) scrapeItems(ctx context.Context) ([]scraperResult, error) {
	job := newJob(func(item scraperItem) (scraperResult, error) {
		return widget.scrapeItemTask(ctx, item)
	}, widget.Items).withWorkers(3)

	results, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	failed := 0
	for i := range results {
		if errs[i] != nil {
			failed++
			results[i].Error = errs[i].Error()
			slog.Error("Failed to scrape item", "url", widget.Items[i].URL, "error", errs[i])
		}
	}

	// If all items failed, return errNoContent to avoid caching
	if failed == len(widget.Items) {
		return nil, errNoContent
	}

	// If some failed, return partial content error
	if failed > 0 {
		return results, errPartialContent
	}

	return results, nil
}

func (widget *scraperWidget) scrapeItemTask(ctx context.Context, item scraperItem) (scraperResult, error) {
	// Rate limiting: delay between requests to avoid overwhelming servers
	time.Sleep(1 * time.Second)

	result := scraperResult{
		Title:      item.Title,
		URL:        item.URL,
		SingleLine: widget.SingleLine,
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", item.URL, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgentString)

	// Execute request
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit response size to prevent memory issues (5MB max)
	limitedBody := io.LimitReader(resp.Body, 5*1024*1024)

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(limitedBody)
	if err != nil {
		return result, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract values for each selector
	values := make([]string, 0, len(item.Selectors))
	for _, selector := range item.Selectors {
		value := widget.extractValue(doc, selector)
		if value != "" {
			values = append(values, value)
		}
	}

	result.Values = values
	return result, nil
}

func (widget *scraperWidget) extractValue(doc *goquery.Document, selector scraperSelector) string {
	selection := doc.Find(selector.Selector).First()
	
	if selection.Length() == 0 {
		return ""
	}

	var value string
	
	// Extract attribute if specified, otherwise extract text
	if selector.Attr != "" {
		value, _ = selection.Attr(selector.Attr)
	} else {
		value = selection.Text()
	}

	// Clean up whitespace
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")

	// Apply prefix and suffix
	if value != "" {
		if selector.Prefix != "" {
			value = selector.Prefix + value
		}
		if selector.Suffix != "" {
			value = value + selector.Suffix
		}
	}

	return value
}

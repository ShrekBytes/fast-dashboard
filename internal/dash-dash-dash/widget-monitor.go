package dashdashdash

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const uptimeHistoryMaxEntries = 10
const uptimeHistoryMaxURLs = 200 // Limit total unique URLs to prevent memory leak

var uptimeHistory = newUptimeHistoryStore(uptimeHistoryMaxEntries)

// Internet connectivity state
var (
	internetAvailableMu    sync.RWMutex
	internetAvailable      bool = true
	lastInternetCheck      time.Time
	internetCheckCacheSecs = 10 // Cache internet status for 10 seconds
)

type uptimeHistoryStore struct {
	mu         sync.Mutex
	max        int
	maxURLs    int
	store      map[string][]int
	accessTime map[string]time.Time // Track last access for LRU eviction
}

func newUptimeHistoryStore(maxEntries int) *uptimeHistoryStore {
	return &uptimeHistoryStore{
		max:        maxEntries,
		maxURLs:    uptimeHistoryMaxURLs,
		store:      make(map[string][]int),
		accessTime: make(map[string]time.Time),
	}
}

// UptimeStatus constants: 0=unknown, 1=up, 2=down
const (
	UptimeUnknown = 0
	UptimeUp      = 1
	UptimeDown    = 2
)

func (u *uptimeHistoryStore) record(url string, status int) {
	// Validate status value
	if status < UptimeUnknown || status > UptimeDown {
		status = UptimeUnknown
	}
	
	u.mu.Lock()
	defer u.mu.Unlock()
	
	// Evict oldest URLs if we exceed the limit (LRU eviction)
	if _, exists := u.store[url]; !exists && len(u.store) >= u.maxURLs {
		var oldestURL string
		var oldestTime time.Time
		first := true
		
		for k, t := range u.accessTime {
			if first || t.Before(oldestTime) {
				oldestURL = k
				oldestTime = t
				first = false
			}
		}
		
		if oldestURL != "" {
			delete(u.store, oldestURL)
			delete(u.accessTime, oldestURL)
		}
	}
	
	list := u.store[url]
	list = append(list, status)
	if len(list) > u.max {
		list = list[len(list)-u.max:]
	}
	u.store[url] = list
	u.accessTime[url] = time.Now()
}

func (u *uptimeHistoryStore) get(url string) []int {
	u.mu.Lock()
	defer u.mu.Unlock()
	list := u.store[url]
	result := make([]int, len(list))
	copy(result, list)
	
	// Update access time for LRU
	if len(list) > 0 {
		u.accessTime[url] = time.Now()
	}
	
	return result
}

var (
	monitorWidgetTemplate        = mustParseTemplate("monitor.html", "widget-base.html")
	monitorWidgetCompactTemplate = mustParseTemplate("monitor-compact.html", "widget-base.html")
)

type monitorWidget struct {
	widgetBase `yaml:",inline"`
	Sites      []struct {
		*SiteStatusRequest `yaml:",inline"`
		Status             *siteStatus     `yaml:"-"`
		URL                string          `yaml:"-"`
		ErrorURL           string          `yaml:"error-url"`
		Title              string          `yaml:"title"`
		Icon               customIconField `yaml:"icon"`
		SameTab            bool            `yaml:"same-tab"`
		StatusText         string          `yaml:"-"`
		StatusStyle        string          `yaml:"-"`
		AltStatusCodes     []int           `yaml:"alt-status-codes"`
		History            []int           `yaml:"-"` // last N uptime status: 0=unknown, 1=up, 2=down
		IsLocal            bool            `yaml:"-"` // true if site is on local network
	} `yaml:"sites"`
	Style               string `yaml:"style"`
	ShowFailingOnly     bool   `yaml:"show-failing-only"`
	ShowInternetStatus  bool   `yaml:"show-internet-status"`
	HasFailing          bool   `yaml:"-"`
	InternetStatus      *siteStatus `yaml:"-"`
	InternetAvailable   bool        `yaml:"-"`
}
func (widget *monitorWidget) IsRefreshable() bool {
	return true
}
func (widget *monitorWidget) initialize() error {
	widget.withTitle("Monitor").withCacheDuration(5 * time.Minute)
	
	// Validate and determine which sites are local
	for i := range widget.Sites {
		site := &widget.Sites[i]
		
		// Validate URL format
		if site.DefaultURL == "" {
			return fmt.Errorf("site %d: url is required", i+1)
		}
		
		// Ensure URL has a scheme
		if !strings.HasPrefix(site.DefaultURL, "http://") && !strings.HasPrefix(site.DefaultURL, "https://") {
			return fmt.Errorf("site %d (%s): url must start with http:// or https://", i+1, site.Title)
		}
		
		// Validate check-url if provided
		if site.CheckURL != "" {
			if !strings.HasPrefix(site.CheckURL, "http://") && !strings.HasPrefix(site.CheckURL, "https://") {
				return fmt.Errorf("site %d (%s): check-url must start with http:// or https://", i+1, site.Title)
			}
		}
		
		site.IsLocal = isLocalURL(site.DefaultURL)
	}
	
	return nil
}

func (widget *monitorWidget) update(ctx context.Context) {
	// Check internet connectivity
	internetUp := checkInternetConnectivity()
	widget.InternetAvailable = internetUp

	// Check internet status for display if enabled
	if widget.ShowInternetStatus {
		if internetUp {
			widget.InternetStatus = &siteStatus{
				Code:         200,
				ResponseTime: 0,
				Error:        nil,
			}
		} else {
			widget.InternetStatus = &siteStatus{
				Code:  0,
				Error: errors.New("no internet connection"),
			}
		}
	}

	// Determine which sites to check based on internet availability
	var requestsToCheck []*SiteStatusRequest
	var indicesToCheck []int

	for i := range widget.Sites {
		site := &widget.Sites[i]
		if internetUp || site.IsLocal {
			requestsToCheck = append(requestsToCheck, site.SiteStatusRequest)
			indicesToCheck = append(indicesToCheck, i)
		}
	}

	if len(requestsToCheck) > 0 {
		statuses, err := fetchStatusForSites(requestsToCheck)
		if !widget.canContinueUpdateAfterHandlingErr(err) {
			return
		}

		// Update checked sites
		for j, i := range indicesToCheck {
			site := &widget.Sites[i]
			status := &statuses[j]
			site.Status = status

			isUp := (status.Code == 200 || slices.Contains(site.AltStatusCodes, status.Code)) && status.Error == nil
			uptimeStatus := UptimeDown
			if isUp {
				uptimeStatus = UptimeUp
			}
			uptimeHistory.record(site.DefaultURL, uptimeStatus)
			site.History = uptimeHistory.get(site.DefaultURL)

			if status.Error != nil && site.ErrorURL != "" {
				site.URL = site.ErrorURL
			} else {
				site.URL = site.DefaultURL
			}
			site.StatusText = statusCodeToText(status.Code, status.TimedOut, status.Error, site.AltStatusCodes)
			site.StatusStyle = statusCodeToStyle(status.Code, status.TimedOut, status.Error, site.AltStatusCodes)
		}
	}

	// Handle remote sites when internet is down (don't check, mark as unknown)
	for i := range widget.Sites {
		site := &widget.Sites[i]
		if !internetUp && !site.IsLocal {
			// Record unknown state in history
			uptimeHistory.record(site.DefaultURL, UptimeUnknown)
			site.History = uptimeHistory.get(site.DefaultURL)
			
			if site.Status == nil {
				site.Status = &siteStatus{}
			}
			site.StatusText = "Unknown"
			site.StatusStyle = "unknown"
			site.URL = site.DefaultURL
		}
	}

	// Check if any sites are failing
	widget.HasFailing = false
	for i := range widget.Sites {
		site := &widget.Sites[i]
		if site.StatusStyle == "error" {
			widget.HasFailing = true
			break
		}
	}

	// Adjust cache duration if internet is down (check more frequently)
	if !internetUp {
		widget.withCacheDuration(60 * time.Second) // Check every minute when internet is down
	} else {
		widget.withCacheDuration(5 * time.Minute) // Normal 5-minute interval
	}

	widget.withError(nil).scheduleNextUpdate()
}

func (widget *monitorWidget) Render() template.HTML {
	if widget.Style == "compact" {
		return widget.renderTemplate(widget, monitorWidgetCompactTemplate)
	}
	return widget.renderTemplate(widget, monitorWidgetTemplate)
}

func statusCodeToText(status int, timedOut bool, err error, altStatusCodes []int) string {
	// Handle timeout
	if timedOut {
		return "Timeout"
	}

	// Handle connection errors (no status code)
	if err != nil && status == 0 {
		return "Connection Error"
	}

	// Handle status codes - always show the actual code
	if status > 0 {
		return strconv.Itoa(status)
	}

	return "Unknown"
}

func statusCodeToStyle(status int, timedOut bool, err error, altStatusCodes []int) string {
	// Unknown status (no error, no code) - shouldn't happen but handle it
	if status == 0 && err == nil {
		return "unknown"
	}

	// Success: 200 or in alt status codes list
	if status == 200 || slices.Contains(altStatusCodes, status) {
		return "ok"
	}

	// Error: timeout, connection error, or bad status code
	if timedOut || err != nil || status >= 400 {
		return "error"
	}

	// Other 2xx, 3xx codes
	if status >= 200 && status < 400 {
		return "ok"
	}

	return "error"
}

type SiteStatusRequest struct {
	DefaultURL    string        `yaml:"url"`
	CheckURL      string        `yaml:"check-url"`
	AllowInsecure bool          `yaml:"allow-insecure"`
	Timeout       durationField `yaml:"timeout"`
	BasicAuth     struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"basic-auth"`
}

type siteStatus struct {
	Code         int
	TimedOut     bool
	ResponseTime time.Duration
	Error        error
}

func fetchSiteStatusTask(statusRequest *SiteStatusRequest) (siteStatus, error) {
	var url string
	if statusRequest.CheckURL != "" {
		url = statusRequest.CheckURL
	} else {
		url = statusRequest.DefaultURL
	}

	timeout := ternary(statusRequest.Timeout > 0, time.Duration(statusRequest.Timeout), 7*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return siteStatus{Error: err}, nil
	}
	if statusRequest.BasicAuth.Username != "" || statusRequest.BasicAuth.Password != "" {
		request.SetBasicAuth(statusRequest.BasicAuth.Username, statusRequest.BasicAuth.Password)
	}

	requestSentAt := time.Now()
	var response *http.Response
	if !statusRequest.AllowInsecure {
		response, err = monitorHTTPClient.Do(request)
	} else {
		response, err = monitorInsecureHTTPClient.Do(request)
	}

	status := siteStatus{ResponseTime: time.Since(requestSentAt)}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			status.TimedOut = true
		}
		status.Error = err
		return status, nil
	}
	defer response.Body.Close()
	status.Code = response.StatusCode
	return status, nil
}

func fetchStatusForSites(requests []*SiteStatusRequest) ([]siteStatus, error) {
	// Scale workers dynamically: 1-20 based on site count
	workerCount := min(20, max(1, len(requests)))
	job := newJob(fetchSiteStatusTask, requests).withWorkers(workerCount)
	results, _, err := workerPoolDo(job)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// checkInternetConnectivity checks if internet is available
// Uses cached result if checked recently
func checkInternetConnectivity() bool {
	internetAvailableMu.RLock()
	if time.Since(lastInternetCheck) < time.Duration(internetCheckCacheSecs)*time.Second {
		result := internetAvailable
		internetAvailableMu.RUnlock()
		return result
	}
	internetAvailableMu.RUnlock()

	// Need to check - upgrade to write lock
	internetAvailableMu.Lock()
	defer internetAvailableMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(lastInternetCheck) < time.Duration(internetCheckCacheSecs)*time.Second {
		return internetAvailable
	}

	// Try multiple privacy-focused endpoints using HTTP HEAD (reuses connections)
	endpoints := []string{
		"https://1.1.1.1",      // Cloudflare
		"https://dns.quad9.net", // Quad9
	}

	for _, endpoint := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
		if err != nil {
			cancel()
			continue
		}
		
		resp, err := defaultHTTPClient.Do(req)
		cancel() // Always cancel context to free resources
		
		if err == nil {
			resp.Body.Close()
			internetAvailable = true
			lastInternetCheck = time.Now()
			return true
		}
	}

	// All checks failed - internet is down
	internetAvailable = false
	lastInternetCheck = time.Now()
	return false
}

// isLocalURL determines if a URL points to a local/private network
func isLocalURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	host := parsedURL.Hostname()

	// Check for localhost variants
	if host == "localhost" || host == "" {
		return true
	}

	// Parse IP address
	ip := net.ParseIP(host)
	if ip == nil {
		// Try to resolve hostname
		addrs, err := net.LookupIP(host)
		if err != nil || len(addrs) == 0 {
			return false
		}
		ip = addrs[0]
	}

	// Check if it's a loopback address
	if ip.IsLoopback() {
		return true
	}

	// Check for private IP ranges
	if ip.IsPrivate() {
		return true
	}

	// Additional check for 0.0.0.0
	if ip.String() == "0.0.0.0" || ip.String() == "::" {
		return true
	}

	return false
}

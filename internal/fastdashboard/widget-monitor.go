package fastdashboard

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

const uptimeHistoryMaxEntries = 10

var uptimeHistory = newUptimeHistoryStore(uptimeHistoryMaxEntries)

type uptimeHistoryStore struct {
	mu    sync.Mutex
	max   int
	store map[string][]bool
}

func newUptimeHistoryStore(maxEntries int) *uptimeHistoryStore {
	return &uptimeHistoryStore{max: maxEntries, store: make(map[string][]bool)}
}

func (u *uptimeHistoryStore) record(url string, isUp bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	list := u.store[url]
	list = append(list, isUp)
	if len(list) > u.max {
		list = list[len(list)-u.max:]
	}
	u.store[url] = list
}

func (u *uptimeHistoryStore) get(url string) []bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	list := u.store[url]
	result := make([]bool, len(list))
	copy(result, list)
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
		History            []bool          `yaml:"-"` // last N up/down results for uptime dots
	} `yaml:"sites"`
	Style           string `yaml:"style"`
	ShowFailingOnly bool   `yaml:"show-failing-only"`
	HasFailing      bool   `yaml:"-"`
}

func (widget *monitorWidget) initialize() error {
	widget.withTitle("Monitor").withCacheDuration(5 * time.Minute)
	return nil
}

func (widget *monitorWidget) update(ctx context.Context) {
	requests := make([]*SiteStatusRequest, len(widget.Sites))
	for i := range widget.Sites {
		requests[i] = widget.Sites[i].SiteStatusRequest
	}

	statuses, err := fetchStatusForSites(requests)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.HasFailing = false
	for i := range widget.Sites {
		site := &widget.Sites[i]
		status := &statuses[i]
		site.Status = status

		isUp := (status.Code == 200 || slices.Contains(site.AltStatusCodes, status.Code)) && status.Error == nil
		uptimeHistory.record(site.DefaultURL, isUp)
		site.History = uptimeHistory.get(site.DefaultURL)

		if !slices.Contains(site.AltStatusCodes, status.Code) && (status.Code >= 400 || status.Error != nil) {
			widget.HasFailing = true
		}
		if status.Error != nil && site.ErrorURL != "" {
			site.URL = site.ErrorURL
		} else {
			site.URL = site.DefaultURL
		}
		site.StatusText = statusCodeToText(status.Code, site.AltStatusCodes)
		site.StatusStyle = statusCodeToStyle(status.Code, site.AltStatusCodes)
	}
	widget.withError(nil).scheduleNextUpdate()
}

func (widget *monitorWidget) Render() template.HTML {
	if widget.Style == "compact" {
		return widget.renderTemplate(widget, monitorWidgetCompactTemplate)
	}
	return widget.renderTemplate(widget, monitorWidgetTemplate)
}

func statusCodeToText(status int, altStatusCodes []int) string {
	if status == 200 || slices.Contains(altStatusCodes, status) {
		return "OK"
	}
	if status == 404 {
		return "Not Found"
	}
	if status == 403 {
		return "Forbidden"
	}
	if status == 401 {
		return "Unauthorized"
	}
	if status >= 500 {
		return "Server Error"
	}
	if status >= 400 {
		return "Client Error"
	}
	return strconv.Itoa(status)
}

func statusCodeToStyle(status int, altStatusCodes []int) string {
	if status == 200 || slices.Contains(altStatusCodes, status) {
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

	timeout := ternary(statusRequest.Timeout > 0, time.Duration(statusRequest.Timeout), 3*time.Second)
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
		response, err = defaultHTTPClient.Do(request)
	} else {
		response, err = defaultInsecureHTTPClient.Do(request)
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
	job := newJob(fetchSiteStatusTask, requests).withWorkers(20)
	results, _, err := workerPoolDo(job)
	if err != nil {
		return nil, err
	}
	return results, nil
}

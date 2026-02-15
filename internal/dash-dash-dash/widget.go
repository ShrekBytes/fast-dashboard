package dashdashdash

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

var widgetIDCounter atomic.Uint64

func newWidget(widgetType string) (widget, error) {
	if widgetType == "" {
		return nil, errors.New("widget 'type' property is empty or not specified")
	}

	var w widget

	switch widgetType {
	case "clock":
		w = &clockWidget{}
	case "calendar":
		w = &calendarWidget{}
	case "search":
		w = &searchWidget{}
	case "weather":
		w = &weatherWidget{}
	case "to-do":
		w = &todoWidget{}
	case "ip-address":
		w = &ipAddressWidget{}
	case "monitor":
		w = &monitorWidget{}
	case "bookmarks":
		w = &bookmarksWidget{}
	case "rss":
		w = &rssWidget{}
	default:
		return nil, fmt.Errorf("unknown widget type: %s", widgetType)
	}

	w.setID(widgetIDCounter.Add(1))

	return w, nil
}

type widgets []widget

func (w *widgets) UnmarshalYAML(node *yaml.Node) error {
	var nodes []yaml.Node

	if err := node.Decode(&nodes); err != nil {
		return err
	}

	for _, node := range nodes {
		meta := struct {
			Type string `yaml:"type"`
		}{}

		if err := node.Decode(&meta); err != nil {
			return err
		}

		widget, err := newWidget(meta.Type)
		if err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}

		if err = node.Decode(widget); err != nil {
			return err
		}

		*w = append(*w, widget)
	}

	return nil
}

type widget interface {
	Render() template.HTML
	GetType() string
	GetID() uint64
	IsRefreshable() bool

	initialize() error
	requiresUpdate(*time.Time) bool
	setProviders(*widgetProviders)
	update(context.Context)
	setID(uint64)
	handleRequest(w http.ResponseWriter, r *http.Request)
	setHideHeader(bool)
}

type cacheType int

const (
	cacheTypeInfinite cacheType = iota
	cacheTypeDuration
	cacheTypeOnTheHour
)

type widgetBase struct {
	ID                  uint64           `yaml:"-"`
	Providers           *widgetProviders `yaml:"-"`
	Type                string           `yaml:"type"`
	Title               string           `yaml:"title"`
	TitleURL            string           `yaml:"title-url"`
	HideHeader          bool             `yaml:"hide-header"`
	CSSClass            string           `yaml:"css-class"`
	CustomCacheDuration durationField    `yaml:"cache"`
	ContentAvailable    bool             `yaml:"-"`
	WIP                 bool             `yaml:"-"`
	Error               error            `yaml:"-"`
	Notice              error            `yaml:"-"`
	templateBuffer      bytes.Buffer     `yaml:"-"`
	cacheDuration       time.Duration    `yaml:"-"`
	cacheType           cacheType        `yaml:"-"`
	nextUpdate          time.Time        `yaml:"-"`
	updateRetriedTimes  int              `yaml:"-"`
}

type widgetProviders struct {
	assetResolver func(string) string
}

func (w *widgetBase) requiresUpdate(now *time.Time) bool {
	if w.cacheType == cacheTypeInfinite {
		return false
	}

	if w.nextUpdate.IsZero() {
		return true
	}

	return now.After(w.nextUpdate)
}

func (w *widgetBase) IsWIP() bool {
	return w.WIP
}

func (w *widgetBase) update(ctx context.Context) {}

func (w *widgetBase) GetID() uint64 {
	return w.ID
}

func (w *widgetBase) setID(id uint64) {
	w.ID = id
}

func (w *widgetBase) IsRefreshable() bool {
	return false
}

func (w *widgetBase) setHideHeader(value bool) {
	w.HideHeader = value
}

func (w *widgetBase) handleRequest(rw http.ResponseWriter, _ *http.Request) {
	http.Error(rw, "not implemented", http.StatusNotImplemented)
}

func (w *widgetBase) GetType() string {
	return w.Type
}

func (w *widgetBase) setProviders(providers *widgetProviders) {
	w.Providers = providers
}

func (w *widgetBase) renderTemplate(data any, t *template.Template) template.HTML {
	w.templateBuffer.Reset()
	err := t.Execute(&w.templateBuffer, data)
	if err != nil {
		w.ContentAvailable = false
		w.Error = err
		slog.Error("Failed to render template", "error", err)
		w.templateBuffer.Reset()
		// Fallback: avoid re-executing the same template; show a safe message.
		w.templateBuffer.WriteString(`<div class="widget-content padding-inline-widget" style="color: var(--color-negative);">Failed to render widget.</div>`)
	}
	return template.HTML(w.templateBuffer.String())
}

func (w *widgetBase) withTitle(title string) *widgetBase {
	if w.Title == "" {
		w.Title = title
	}
	return w
}

func (w *widgetBase) withTitleURL(titleURL string) *widgetBase {
	if w.TitleURL == "" {
		w.TitleURL = titleURL
	}
	return w
}

func (w *widgetBase) withCacheDuration(duration time.Duration) *widgetBase {
	w.cacheType = cacheTypeDuration

	if duration == -1 || w.CustomCacheDuration == 0 {
		w.cacheDuration = duration
	} else {
		w.cacheDuration = time.Duration(w.CustomCacheDuration)
	}

	return w
}

func (w *widgetBase) withCacheOnTheHour() *widgetBase {
	w.cacheType = cacheTypeOnTheHour
	return w
}

func (w *widgetBase) withNotice(err error) *widgetBase {
	w.Notice = err
	return w
}

func (w *widgetBase) withError(err error) *widgetBase {
	if err == nil && !w.ContentAvailable {
		w.ContentAvailable = true
	}
	w.Error = err
	return w
}

func (w *widgetBase) canContinueUpdateAfterHandlingErr(err error) bool {
	if err != nil {
		w.scheduleEarlyUpdate()

		if !errors.Is(err, errPartialContent) {
			w.withError(err)
			w.withNotice(nil)
			return false
		}

		w.withError(nil)
		w.withNotice(err)
		return true
	}

	w.withNotice(nil)
	w.withError(nil)
	w.scheduleNextUpdate()
	return true
}

func (w *widgetBase) getNextUpdateTime() time.Time {
	now := time.Now()

	if w.cacheType == cacheTypeDuration {
		return now.Add(w.cacheDuration)
	}

	if w.cacheType == cacheTypeOnTheHour {
		return now.Add(time.Duration(
			((60-now.Minute())*60)-now.Second(),
		) * time.Second)
	}

	return time.Time{}
}

func (w *widgetBase) scheduleNextUpdate() *widgetBase {
	w.nextUpdate = w.getNextUpdateTime()
	w.updateRetriedTimes = 0
	return w
}

func (w *widgetBase) scheduleEarlyUpdate() *widgetBase {
	w.updateRetriedTimes++

	if w.updateRetriedTimes > 5 {
		w.updateRetriedTimes = 5
	}

	nextEarlyUpdate := time.Now().Add(time.Duration(math.Pow(float64(w.updateRetriedTimes), 2)) * time.Minute)
	nextUsualUpdate := w.getNextUpdateTime()

	if nextEarlyUpdate.After(nextUsualUpdate) {
		w.nextUpdate = nextUsualUpdate
	} else {
		w.nextUpdate = nextEarlyUpdate
	}

	return w
}

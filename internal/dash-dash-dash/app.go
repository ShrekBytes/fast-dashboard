package dashdashdash

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var buildVersion = "dev"

var (
	pageTemplate        = mustParseTemplate("page.html", "document.html", "footer.html")
	pageContentTemplate = mustParseTemplate("page-content.html")
	manifestTemplate    = mustParseTemplate("manifest.json")
)

const STATIC_ASSETS_CACHE_DURATION = 24 * time.Hour

var reservedPageSlugs = []string{"login", "logout"}

type application struct {
	Version   string
	CreatedAt time.Time
	Config    config

	parsedManifest []byte

	slugToPage    map[string]*page
	widgetByID    map[uint64]widget
	refreshCancel context.CancelFunc
}

func newApplication(c *config) (*application, error) {
	app := &application{
		Version:    buildVersion,
		CreatedAt:  time.Now(),
		Config:     *c,
		slugToPage: make(map[string]*page),
		widgetByID: make(map[uint64]widget),
	}
	config := &app.Config

	config.Theme.Key = "default"
	if err := config.Theme.init(); err != nil {
		return nil, fmt.Errorf("initializing default theme: %v", err)
	}

	app.slugToPage[""] = &config.Pages[0]

	providers := &widgetProviders{
		assetResolver: app.StaticAssetPath,
	}

	for p := range config.Pages {
		page := &config.Pages[p]
		page.PrimaryColumnIndex = -1

		if page.Slug == "" {
			page.Slug = titleToSlug(page.Title)
		}

		if slices.Contains(reservedPageSlugs, page.Slug) {
			return nil, fmt.Errorf("page slug \"%s\" is reserved", page.Slug)
		}

		app.slugToPage[page.Slug] = page

		if page.Width == "default" {
			page.Width = ""
		}

		if page.DesktopNavigationWidth == "" || page.DesktopNavigationWidth == "default" {
			page.DesktopNavigationWidth = page.Width
		}

		for i := range page.HeadWidgets {
			widget := page.HeadWidgets[i]
			app.widgetByID[widget.GetID()] = widget
			widget.setProviders(providers)
		}

		for c := range page.Columns {
			column := &page.Columns[c]

			if page.PrimaryColumnIndex == -1 && column.Size == "full" {
				page.PrimaryColumnIndex = int8(c)
			}

			for w := range column.Widgets {
				widget := column.Widgets[w]
				app.widgetByID[widget.GetID()] = widget
				widget.setProviders(providers)
			}
		}
	}

	config.Server.BaseURL = strings.TrimRight(config.Server.BaseURL, "/")
	if u, err := url.Parse(config.Server.BaseURL); err == nil {
		config.Server.BasePath = strings.TrimSuffix(u.Path, "/")
	}
	config.Theme.CustomCSSFile = app.resolveUserDefinedAssetPath(config.Theme.CustomCSSFile)
	config.Branding.LogoURL = app.resolveUserDefinedAssetPath(config.Branding.LogoURL)

	config.Branding.FaviconURL = ternary(
		config.Branding.FaviconURL == "",
		app.StaticAssetPath("favicon.svg"),
		app.resolveUserDefinedAssetPath(config.Branding.FaviconURL),
	)

	config.Branding.FaviconType = ternary(
		strings.HasSuffix(config.Branding.FaviconURL, ".svg"),
		"image/svg+xml",
		"image/png",
	)

	if config.Branding.AppName == "" {
		config.Branding.AppName = "DASH-DASH-DASH"
	}

	if config.Branding.AppIconURL == "" {
		config.Branding.AppIconURL = app.StaticAssetPath("app-icon.png")
	}

	if config.Branding.AppBackgroundColor == "" {
		config.Branding.AppBackgroundColor = config.Theme.BackgroundColorAsHex
	}

	manifest, err := executeTemplateToString(manifestTemplate, templateData{App: app})
	if err != nil {
		return nil, fmt.Errorf("parsing manifest.json: %v", err)
	}
	app.parsedManifest = []byte(manifest)

	return app, nil
}

func (p *page) updateOutdatedWidgets() {
	now := time.Now()

	var wg sync.WaitGroup
	ctx := context.Background()

	for w := range p.HeadWidgets {
		widget := p.HeadWidgets[w]

		if !widget.requiresUpdate(&now) {
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p.HeadWidgets[idx].update(ctx)
		}(w)
	}

	for c := range p.Columns {
		for w := range p.Columns[c].Widgets {
			widget := p.Columns[c].Widgets[w]

			if !widget.requiresUpdate(&now) {
				continue
			}

			wg.Add(1)
			go func(colIdx, widIdx int) {
				defer wg.Done()
				p.Columns[colIdx].Widgets[widIdx].update(ctx)
			}(c, w)
		}
	}

	wg.Wait()
}

func (a *application) resolveUserDefinedAssetPath(path string) string {
	if strings.HasPrefix(path, "/assets/") {
		return a.Config.Server.BaseURL + path
	}

	return path
}

type templateRequestData struct {
	Theme *themeProperties
}

type templateData struct {
	App     *application
	Page    *page
	Request templateRequestData
}

func (a *application) populateTemplateRequestData(data *templateRequestData, _ *http.Request) {
	data.Theme = &a.Config.Theme.themeProperties
}

func (a *application) handlePageRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]
	if !exists {
		a.handleNotFound(w, r)
		return
	}

	data := templateData{
		Page: page,
		App:  a,
	}
	a.populateTemplateRequestData(&data.Request, r)

	var responseBytes bytes.Buffer
	err := pageTemplate.Execute(&responseBytes, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Write(responseBytes.Bytes())
}

func (a *application) handlePageContentRequest(w http.ResponseWriter, r *http.Request) {
	page, exists := a.slugToPage[r.PathValue("page")]
	if !exists {
		a.handleNotFound(w, r)
		return
	}

	pageData := templateData{
		Page: page,
	}

	var err error
	var responseBytes bytes.Buffer

	func() {
		page.mu.Lock()
		defer page.mu.Unlock()

		page.updateOutdatedWidgets()
		err = pageContentTemplate.Execute(&responseBytes, pageData)
	}()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(responseBytes.Bytes())
}

func (a *application) handleFaviconRedirect(w http.ResponseWriter, r *http.Request) {
	dest := a.Config.Branding.FaviconURL
	if dest == "" {
		dest = a.StaticAssetPath("favicon.svg")
	}
	if dest != "" && !strings.HasPrefix(dest, "http://") && !strings.HasPrefix(dest, "https://") {
		dest = strings.TrimRight(a.Config.Server.BaseURL, "/") + "/" + strings.TrimPrefix(dest, "/")
	}
	if dest == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	http.Redirect(w, r, dest, http.StatusMovedPermanently)
}

func (a *application) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Page not found"))
}

func (a *application) handleWidgetRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

const (
	backgroundRefreshInterval = 5 * time.Minute
	backgroundRefreshInitial = 2 * time.Second
)

func (a *application) runBackgroundRefresh(ctx context.Context) {
	// Initial warm-up so the first page load has fresh data
	select {
	case <-ctx.Done():
		return
	case <-time.After(backgroundRefreshInitial):
		a.refreshAllPages()
	}

	ticker := time.NewTicker(backgroundRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.refreshAllPages()
		}
	}
}

func (a *application) refreshAllPages() {
	for _, page := range a.slugToPage {
		page.mu.Lock()
		page.updateOutdatedWidgets()
		page.mu.Unlock()
	}
}

func (a *application) StaticAssetPath(asset string) string {
	return a.Config.Server.BasePath + "/static/" + staticFSHash + "/" + asset
}

func (a *application) VersionedAssetPath(asset string) string {
	return a.Config.Server.BasePath + asset +
		"?v=" + strconv.FormatInt(a.CreatedAt.Unix(), 10)
}

func (a *application) server() (func() error, func() error) {
	mux := http.NewServeMux()

	// API routes first so /api/... is never matched by GET /{page}
	mux.HandleFunc("GET /api/pages/{page}/content/", a.handlePageContentRequest) // {page} can be "" for root

	mux.HandleFunc("GET /favicon.ico", a.handleFaviconRedirect)

	mux.HandleFunc("GET /{$}", a.handlePageRequest)
	mux.HandleFunc("GET /{page}", a.handlePageRequest)

	mux.HandleFunc("/api/widgets/{widget}/{path...}", a.handleWidgetRequest)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle(
		fmt.Sprintf("GET /static/%s/{path...}", staticFSHash),
		http.StripPrefix(
			"/static/"+staticFSHash,
			fileServerWithCache(http.FS(staticFS), STATIC_ASSETS_CACHE_DURATION),
		),
	)

	assetCacheControlValue := fmt.Sprintf(
		"public, max-age=%d",
		int(STATIC_ASSETS_CACHE_DURATION.Seconds()),
	)

	mux.HandleFunc(fmt.Sprintf("GET /static/%s/css/bundle.css", staticFSHash), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", assetCacheControlValue)
		w.Header().Add("Content-Type", "text/css; charset=utf-8")
		w.Write(bundledCSSContents)
	})

	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", assetCacheControlValue)
		w.Header().Add("Content-Type", "application/json")
		w.Write(a.parsedManifest)
	})

	var absAssetsPath string
	if a.Config.Server.AssetsPath != "" {
		absAssetsPath, _ = filepath.Abs(a.Config.Server.AssetsPath)
		assetsFS := fileServerWithCache(http.Dir(a.Config.Server.AssetsPath), 2*time.Hour)
		mux.Handle("/assets/{path...}", http.StripPrefix("/assets/", assetsFS))
	}

	server := http.Server{
		Addr:    fmt.Sprintf("%s:%d", a.Config.Server.Host, a.Config.Server.Port),
		Handler: mux,
	}

	refreshCtx, refreshCancel := context.WithCancel(context.Background())
	a.refreshCancel = refreshCancel
	go a.runBackgroundRefresh(refreshCtx)

	start := func() error {
		log.Printf("Starting server on %s:%d (base-url: \"%s\", assets-path: \"%s\")\n",
			a.Config.Server.Host,
			a.Config.Server.Port,
			a.Config.Server.BaseURL,
			absAssetsPath,
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}

		return nil
	}

	stop := func() error {
		a.refreshCancel()
		return server.Close()
	}

	return start, stop
}

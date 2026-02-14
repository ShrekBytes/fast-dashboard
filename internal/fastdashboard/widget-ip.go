package fastdashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ipAddressWidgetTemplate = mustParseTemplate("ip-address.html", "widget-base.html")

const defaultPublicIPURL = "https://ipinfo.io/json"
const defaultIPGeoURL = "http://ip-api.com/json/%s?fields=countryCode"
const defaultIPCacheDuration = 10 * time.Minute

type ipAddressWidget struct {
	widgetBase  `yaml:",inline"`
	PublicURL   *string      `yaml:"public-url"` // nil = omitted = use default; non-nil empty = hide public IP; non-nil with URL = use that URL
	Interfaces  []string     `yaml:"interfaces"`
	Hostname    string       `yaml:"-"`
	LocalIPs    []ipAddrLine `yaml:"-"`
	PublicIP    string       `yaml:"-"`
	PublicLabel string       `yaml:"-"`
}

type ipAddrLine struct {
	Label string
	Value string
}

func (widget *ipAddressWidget) initialize() error {
	widget.withTitle("IP Address").withError(nil)
	// When public-url is omitted (nil), use default so public IP is shown. Set to "" to hide, or to a URL to use that endpoint.
	if widget.PublicURL == nil {
		u := defaultPublicIPURL
		widget.PublicURL = &u
	}
	if widget.CustomCacheDuration == 0 {
		widget.withCacheDuration(defaultIPCacheDuration)
	} else {
		widget.withCacheDuration(time.Duration(widget.CustomCacheDuration))
	}
	return nil
}

// getDefaultRouteInterface returns the default route interface name (e.g. "wlo1", "eth0") on Linux via "ip route get 8.8.8.8". Returns "" on non-Linux or on failure.
func getDefaultRouteInterface() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ip", "-o", "route", "get", "8.8.8.8")
	cmd.Env = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	parts := strings.Fields(string(out))
	for i := range parts {
		if i+1 < len(parts) && parts[i] == "dev" {
			return parts[i+1]
		}
	}
	return ""
}

// getActiveLocalIP returns the local IP that would be used to reach the internet (e.g. 8.8.8.8). Works on all platforms.
func getActiveLocalIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 2*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr == nil {
		return ""
	}
	if ip := addr.IP.To4(); ip != nil {
		return ip.String()
	}
	return addr.IP.String()
}

func (widget *ipAddressWidget) update(ctx context.Context) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	widget.Hostname = hostname

	// Only the active local IP (default-route interface) + public IP, matching Python behavior.
	activeIP := getActiveLocalIP()
	ifaceName := getDefaultRouteInterface()
	if widget.Interfaces != nil && len(widget.Interfaces) > 0 {
		// Optional filter: if user specified interfaces, only show if the default interface is in the list.
		if ifaceName != "" {
			allowed := make(map[string]bool)
			for _, n := range widget.Interfaces {
				allowed[n] = true
			}
			if !allowed[ifaceName] {
				ifaceName = ""
			}
		}
	}
	localIPs := make([]ipAddrLine, 0, 1)
	if activeIP != "" {
		localIPs = append(localIPs, ipAddrLine{Label: ifaceName, Value: activeIP})
	}
	widget.LocalIPs = localIPs
	widget.withError(nil)

	publicURL := ""
	if widget.PublicURL != nil {
		publicURL = *widget.PublicURL
	}
	if publicURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", publicURL, nil)
		if err != nil {
			widget.PublicIP = ""
			widget.PublicLabel = ""
			widget.canContinueUpdateAfterHandlingErr(err)
			return
		}
		resp, err := defaultHTTPClient.Do(req)
		if err != nil {
			widget.PublicIP = ""
			widget.PublicLabel = ""
			widget.canContinueUpdateAfterHandlingErr(err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			widget.PublicIP = ""
			widget.PublicLabel = ""
			widget.scheduleNextUpdate()
			return
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			widget.PublicIP = ""
			widget.PublicLabel = ""
			widget.scheduleNextUpdate()
			return
		}
		// If response is JSON with "ip" (e.g. ipinfo.io/json), use it and optionally "country" — one call instead of two.
		if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "application/json") {
			var info struct {
				IP      string `json:"ip"`
				Country string `json:"country"`
			}
			if jsonErr := json.Unmarshal(body, &info); jsonErr == nil && info.IP != "" {
				widget.PublicIP = strings.TrimSpace(info.IP)
				widget.PublicLabel = "Public"
				if info.Country != "" {
					widget.PublicLabel = "Public (" + info.Country + ")"
				}
				widget.withError(nil).scheduleNextUpdate()
				return
			}
		}
		ipStr := strings.TrimSpace(string(body))
		widget.PublicIP = ipStr
		widget.PublicLabel = "Public"
		if cc := widget.fetchCountryCode(ctx, ipStr); cc != "" {
			widget.PublicLabel = "Public (" + cc + ")"
		}
		widget.withError(nil).scheduleNextUpdate()
	} else {
		widget.PublicIP = ""
		widget.PublicLabel = ""
		widget.scheduleNextUpdate()
	}
}

func (widget *ipAddressWidget) fetchCountryCode(ctx context.Context, ip string) string {
	url := fmt.Sprintf(defaultIPGeoURL, ip)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var v struct {
		CountryCode string `json:"countryCode"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128)).Decode(&v); err != nil {
		return ""
	}
	return v.CountryCode
}

func (widget *ipAddressWidget) Render() template.HTML {
	return widget.renderTemplate(widget, ipAddressWidgetTemplate)
}

package dashdashdash

import (
	"html/template"
	"math"
	"net/url"
	"strconv"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var intl = message.NewPrinter(language.English)

var globalTemplateFunctions = template.FuncMap{
	"formatNumber": intl.Sprint,
	"safeCSS": func(str string) template.CSS {
		return template.CSS(str)
	},
	"safeURL": func(str string) template.URL {
		return template.URL(str)
	},
	"safeHTML": func(str string) template.HTML {
		return template.HTML(str)
	},
	"absInt": func(i int) int {
		return int(math.Abs(float64(i)))
	},
	"dynamicRelativeTimeAttrs": dynamicRelativeTimeAttrs,
	"faviconURLFor": faviconURLFor,
}

func mustParseTemplate(primary string, dependencies ...string) *template.Template {
	t, err := template.New(primary).
		Funcs(globalTemplateFunctions).
		ParseFS(templateFS, append([]string{primary}, dependencies...)...)

	if err != nil {
		panic(err)
	}

	return t
}

func dynamicRelativeTimeAttrs(t interface{ Unix() int64 }) template.HTMLAttr {
	return template.HTMLAttr(`data-dynamic-relative-time="` + strconv.FormatInt(t.Unix(), 10) + `"`)
}

func faviconURLFor(linkURL string) string {
	domain := extractDomainFromUrl(linkURL)
	if domain == "" {
		return ""
	}
	// Use DuckDuckGo's favicon service; Google's (s2/favicons → t2.gstatic.com) often returns 404.
	return "https://icons.duckduckgo.com/ip3/" + url.PathEscape(domain) + ".ico"
}

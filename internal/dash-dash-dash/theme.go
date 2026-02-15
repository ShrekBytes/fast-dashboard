package dashdashdash

import (
	"fmt"
	"html/template"
)

var themeStyleTemplate = mustParseTemplate("theme-style.gotmpl")

type themeProperties struct {
	BackgroundColor          *hslColorField `yaml:"background-color"`
	PrimaryColor             *hslColorField `yaml:"primary-color"`
	PositiveColor            *hslColorField `yaml:"positive-color"`
	NegativeColor            *hslColorField `yaml:"negative-color"`
	Light                    bool           `yaml:"light"`
	ContrastMultiplier       float32        `yaml:"contrast-multiplier"`
	TextSaturationMultiplier float32        `yaml:"text-saturation-multiplier"`

	Key                  string       `yaml:"-"`
	CSS                  template.CSS `yaml:"-"`
	BackgroundColorAsHex  string       `yaml:"-"`
}

func (t *themeProperties) init() error {
	css, err := executeTemplateToString(themeStyleTemplate, t)
	if err != nil {
		return fmt.Errorf("compiling theme style: %v", err)
	}
	t.CSS = template.CSS(whitespaceAtBeginningOfLinePattern.ReplaceAllString(css, ""))

	if t.BackgroundColor != nil {
		t.BackgroundColorAsHex = t.BackgroundColor.ToHex()
	} else {
		t.BackgroundColorAsHex = "#151519"
	}

	return nil
}

package web

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
)

var (
	md        = goldmark.New()
	sanitizer = bluemonday.UGCPolicy()
)

// renderMarkdown converts markdown to sanitised HTML for safe inline display.
// The bluemonday sanitizer is load-bearing: injected script and event-handler
// attributes are stripped before the output is rendered as template.HTML.
func renderMarkdown(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML(sanitizer.Sanitize(src))
	}
	return template.HTML(sanitizer.SanitizeBytes(buf.Bytes()))
}
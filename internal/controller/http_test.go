package controller

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestSafePreviewCSSColor(t *testing.T) {
	if got := string(safePreviewCSSColor("rgb(84, 52, 245)", "#000000")); got != "rgb(84, 52, 245)" {
		t.Fatalf("valid preview color = %q", got)
	}
	if got := string(safePreviewCSSColor("red; background: url(evil)", "#000000")); got != "#000000" {
		t.Fatalf("unsafe preview color = %q, want fallback", got)
	}
}

func TestPreviewCSSDeclarationsRenderWithoutTemplateFilter(t *testing.T) {
	tmpl := template.Must(template.New("preview-style").Parse(`<style>.pass { {{.}} }</style>`))
	var out bytes.Buffer
	if err := tmpl.Execute(&out, template.CSS("background: rgb(84, 52, 245); color: rgb(255, 255, 255);")); err != nil {
		t.Fatalf("render preview style: %v", err)
	}
	if strings.Contains(out.String(), "ZgotmplZ") {
		t.Fatalf("preview style was rejected by template filter: %s", out.String())
	}
}

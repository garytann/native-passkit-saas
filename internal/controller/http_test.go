package controller

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"native-ios-passkit-wip/internal/applepass"
	"native-ios-passkit-wip/internal/service"
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

func TestPreviewTemplateRendersPassWithoutPrimaryFields(t *testing.T) {
	pass := applepass.NewStampCardPass(applepass.TemplateConfig{}, applepass.StampCardDesign{
		ProgramName:     "Coffee Brew Loyalty Card",
		PassDescription: "Collect stamps at Coffee Brew.",
	}, applepass.StampCardMember{ID: "MBR-1001", BarcodeID: "stamp-card:MBR-1001"})
	var out bytes.Buffer
	err := previewTemplate.Execute(&out, previewPageData{
		PassPreview: service.PassPreview{
			Pass:        pass,
			Member:      applepass.StampCardMember{ID: "MBR-1001"},
			Barcode:     pass.Barcodes[0],
			StripBase64: "c3RyaXA=",
		},
		PassColorDeclarations: template.CSS("background: rgb(84, 52, 245); color: rgb(255, 255, 255);"),
		LabelColorDeclaration: template.CSS("color: rgb(225, 220, 255);"),
	})
	if err != nil {
		t.Fatalf("render preview without primary fields: %v", err)
	}
	html := out.String()
	if !strings.Contains(html, "Coffee Brew Loyalty Card") || !strings.Contains(html, "Programme") {
		t.Fatalf("expected configured programme in preview details: %s", html)
	}
	if strings.Contains(html, "About This Programme") {
		t.Fatalf("expected legacy about field to be absent: %s", html)
	}
}

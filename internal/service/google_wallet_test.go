package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestValidateGoogleWalletPublicBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "public HTTPS", value: "https://wallet-assets.example.com", wantErr: false},
		{name: "HTTPS tunnel", value: "https://example.ngrok-free.app", wantErr: false},
		{name: "localhost", value: "https://localhost", wantErr: true},
		{name: "loopback", value: "https://127.0.0.1", wantErr: true},
		{name: "private address", value: "https://192.168.1.20", wantErr: true},
		{name: "plain HTTP", value: "http://wallet-assets.example.com", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateGoogleWalletPublicBaseURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateGoogleWalletPublicBaseURL(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestNormalizeGoogleProgramLogo(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 660, 660))
	source.SetNRGBA(20, 20, color.NRGBA{R: 84, G: 52, B: 245, A: 120})
	normalized, err := normalizeGoogleProgramLogo(testPNGBytes(t, source))
	if err != nil {
		t.Fatalf("normalize program logo: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(normalized))
	if err != nil {
		t.Fatalf("decode normalized logo: %v", err)
	}
	if decoded.Bounds().Dx() != 660 || decoded.Bounds().Dy() != 660 {
		t.Fatalf("normalized dimensions = %v", decoded.Bounds())
	}
}

func TestNormalizeGoogleLogosRejectInvalidDimensions(t *testing.T) {
	if _, err := normalizeGoogleProgramLogo(testPNGBytes(t, image.NewRGBA(image.Rect(0, 0, 660, 640)))); err == nil {
		t.Fatal("expected non-square program logo to fail")
	}
	if _, err := normalizeGoogleWideLogo(testPNGBytes(t, image.NewRGBA(image.Rect(0, 0, 1200, 400)))); err == nil {
		t.Fatal("expected undersized wide logo to fail")
	}
	if _, err := normalizeGoogleWideLogo(testPNGBytes(t, image.NewRGBA(image.Rect(0, 0, 1280, 400)))); err != nil {
		t.Fatalf("expected 1280x400 wide logo to pass: %v", err)
	}
}

func testPNGBytes(t *testing.T, source image.Image) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	return output.Bytes()
}

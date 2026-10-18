package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

func TestValidateAppleCertificateAcceptsMatchingCredential(t *testing.T) {
	certificatePath, keyPath, wwdrPath := writeTestAppleCredential(t, "pass.com.justloyalty.test", "TEAM123456")

	metadata, err := validateAppleCertificate(
		certificatePath,
		keyPath,
		wwdrPath,
		"pass.com.justloyalty.test",
		"TEAM123456",
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("expected matching credential to validate, got %v", err)
	}
	if metadata.Fingerprint == "" || metadata.SerialNumber == "" {
		t.Fatalf("expected certificate metadata, got %+v", metadata)
	}
}

func TestValidateAppleCertificateRejectsWrongPassType(t *testing.T) {
	certificatePath, keyPath, wwdrPath := writeTestAppleCredential(t, "pass.com.justloyalty.test", "TEAM123456")

	_, err := validateAppleCertificate(
		certificatePath,
		keyPath,
		wwdrPath,
		"pass.com.justloyalty.other",
		"TEAM123456",
		time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("expected mismatched Pass Type ID to fail validation")
	}
}

func TestValidatePassAssetBundleAcceptsScaledPNGAssets(t *testing.T) {
	directory := t.TempDir()
	writeTestPNG(t, filepath.Join(directory, "icon.png"), 38, 38)
	writeTestPNG(t, filepath.Join(directory, "icon@2x.png"), 76, 76)
	writeTestPNG(t, filepath.Join(directory, "icon@3x.png"), 114, 114)
	writeTestPNG(t, filepath.Join(directory, "logo.png"), 160, 50)
	writeTestPNG(t, filepath.Join(directory, "logo@2x.png"), 320, 100)
	writeTestPNG(t, filepath.Join(directory, "logo@3x.png"), 480, 150)

	if err := validatePassAssetBundle(directory); err != nil {
		t.Fatalf("expected valid asset bundle, got %v", err)
	}
}

func TestValidatePassAssetBundleRejectsIncorrectScale(t *testing.T) {
	directory := t.TempDir()
	writeTestPNG(t, filepath.Join(directory, "icon.png"), 29, 29)
	writeTestPNG(t, filepath.Join(directory, "icon@2x.png"), 58, 58)
	writeTestPNG(t, filepath.Join(directory, "icon@3x.png"), 86, 86)
	writeTestPNG(t, filepath.Join(directory, "logo.png"), 160, 50)
	writeTestPNG(t, filepath.Join(directory, "logo@2x.png"), 320, 100)
	writeTestPNG(t, filepath.Join(directory, "logo@3x.png"), 480, 150)

	if err := validatePassAssetBundle(directory); err == nil {
		t.Fatal("expected incorrectly scaled asset bundle to fail")
	}
}

func TestStoreGeneratedPassAssetsWritesSourcesAndPreservesOtherAssets(t *testing.T) {
	basePath := t.TempDir()
	bundle := filepath.Join(basePath, "merchant-assets")
	if err := os.MkdirAll(bundle, 0755); err != nil {
		t.Fatal(err)
	}
	googleAssetPath := filepath.Join(bundle, "google-program-logo.png")
	if err := os.WriteFile(googleAssetPath, []byte("google-asset"), 0644); err != nil {
		t.Fatal(err)
	}
	sourceImage := image.NewRGBA(image.Rect(0, 0, 512, 512))
	sourceImage.Set(0, 0, color.NRGBA{A: 255})
	source := testPNGBytes(t, sourceImage)
	service := NewAppleWalletService(config.Config{PassAssetBasePath: basePath}, nil)

	if err := service.StoreGeneratedPassAssets("merchant-assets", source, source, nil); err != nil {
		t.Fatalf("store generated pass assets: %v", err)
	}
	if err := validatePassAssetBundle(bundle); err != nil {
		t.Fatalf("validate generated bundle: %v", err)
	}
	for name, expected := range map[string]image.Point{
		"icon.png":        {X: 38, Y: 38},
		"icon@2x.png":     {X: 76, Y: 76},
		"icon@3x.png":     {X: 114, Y: 114},
		"logo.png":        {X: 160, Y: 50},
		"logo@2x.png":     {X: 320, Y: 100},
		"logo@3x.png":     {X: 480, Y: 150},
		"source-icon.png": {X: 512, Y: 512},
		"source-logo.png": {X: 512, Y: 512},
	} {
		data, err := os.ReadFile(filepath.Join(bundle, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if cfg.Width != expected.X || cfg.Height != expected.Y {
			t.Errorf("%s dimensions = %dx%d, want %dx%d", name, cfg.Width, cfg.Height, expected.X, expected.Y)
		}
	}
	data, err := os.ReadFile(googleAssetPath)
	if err != nil || string(data) != "google-asset" {
		t.Fatalf("Google Wallet asset changed: data=%q err=%v", data, err)
	}
}

func TestStampArtworkReturnsMerchantLogoAndCustomIcon(t *testing.T) {
	basePath := t.TempDir()
	bundle := filepath.Join(basePath, "merchant-assets")
	if err := os.MkdirAll(bundle, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(bundle, "icon@3x.png"), 87, 87)
	writeTestPNG(t, filepath.Join(bundle, "stamp.png"), 128, 128)
	service := NewAppleWalletService(config.Config{PassAssetBasePath: basePath}, nil)

	for source, expectedWidth := range map[string]int{"logo": 87, "custom": 128} {
		data, err := service.StampArtwork("merchant-assets", source)
		if err != nil {
			t.Fatalf("read %s artwork: %v", source, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != expectedWidth {
			t.Fatalf("expected %s artwork width %d, got %d", source, expectedWidth, cfg.Width)
		}
	}
}

func TestValidatePassDesignRequestAcceptsWalletInformation(t *testing.T) {
	input := UpsertApplePassDesignRequest{
		MerchantID:         "MCH-TEST1234",
		PassTypeIdentifier: "pass.com.justloyalty.test",
		ProgramName:        "Test Loyalty",
		PassDescription:    "Collect stamps and enjoy rewards.",
		StampGoal:          10,
		BackgroundColor:    "#5434F5",
		ForegroundColor:    "#FFFFFF",
		LabelColor:         "#E1DCFF",
		Links: []ApplePassLinkRequest{
			{LinkType: "instagram", Label: "Instagram", URL: "https://instagram.com/test"},
			{LinkType: "whatsapp", Label: "WhatsApp", URL: "https://wa.me/6591234567"},
		},
	}
	normalizePassDesignRequest(&input)
	if err := validatePassDesignRequest(input); err != nil {
		t.Fatalf("expected pass information to validate, got %v", err)
	}
	if input.StampInstructions == "" || input.RewardTitle != "Loyalty Reward" || input.RewardDetails == "" {
		t.Fatal("expected information defaults to be populated")
	}
}

func TestValidatePassDesignRequestRejectsLongRewardTitle(t *testing.T) {
	input := validPassDesignRequest()
	input.RewardTitle = "12345678901234567890123456789012345678901"
	if err := validatePassDesignRequest(input); err == nil {
		t.Fatal("expected reward title over 40 characters to fail validation")
	}
}

func TestValidatePassDesignRequestAcceptsFrontCampaignMode(t *testing.T) {
	input := validPassDesignRequest()
	input.CampaignDisplayMode = "front_and_details"
	if err := validatePassDesignRequest(input); err != nil {
		t.Fatalf("expected front campaign mode to validate, got %v", err)
	}
}

func TestValidatePassDesignRequestRejectsUnknownCampaignMode(t *testing.T) {
	input := validPassDesignRequest()
	input.CampaignDisplayMode = "front_only"
	if err := validatePassDesignRequest(input); err == nil {
		t.Fatal("expected unknown campaign mode to fail validation")
	}
}

func TestValidateStampGoalChangeLocksActiveDesign(t *testing.T) {
	err := validateStampGoalChange(domain.ApplePassDesign{Status: "active", StampGoal: 10}, 12)
	if err == nil {
		t.Fatal("expected active design goal change to fail")
	}
}

func TestValidateStampGoalChangeAllowsDraftDesign(t *testing.T) {
	err := validateStampGoalChange(domain.ApplePassDesign{Status: "draft", StampGoal: 10}, 12)
	if err != nil {
		t.Fatalf("expected draft design goal change to succeed, got %v", err)
	}
}

func TestValidatePassDesignRequestRejectsInsecureLink(t *testing.T) {
	input := validPassDesignRequest()
	input.Links = []ApplePassLinkRequest{
		{LinkType: "website", Label: "Website", URL: "http://example.com"},
	}
	normalizePassDesignRequest(&input)
	if err := validatePassDesignRequest(input); err == nil {
		t.Fatal("expected non-HTTPS pass link to fail validation")
	}
}

func validPassDesignRequest() UpsertApplePassDesignRequest {
	return UpsertApplePassDesignRequest{
		MerchantID:          "MCH-TEST1234",
		PassTypeIdentifier:  "pass.com.justloyalty.test",
		ProgramName:         "Test Loyalty",
		PassDescription:     "Collect stamps and enjoy rewards.",
		StampGoal:           10,
		RewardTitle:         "Free Coffee",
		BackgroundColor:     "#5434F5",
		ForegroundColor:     "#FFFFFF",
		LabelColor:          "#E1DCFF",
		CampaignDisplayMode: "details",
		StampIconKey:        "none",
	}
}

func writeTestAppleCredential(t *testing.T, passTypeIdentifier string, teamIdentifier string) (string, string, string) {
	t.Helper()
	now := time.Now().UTC()

	wwdrKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate WWDR key: %v", err)
	}
	wwdrTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Apple Worldwide Developer Relations Certification Authority", OrganizationalUnit: []string{"G4"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	wwdrDER, err := x509.CreateCertificate(rand.Reader, wwdrTemplate, wwdrTemplate, &wwdrKey.PublicKey, wwdrKey)
	if err != nil {
		t.Fatalf("create WWDR certificate: %v", err)
	}

	passKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate pass key: %v", err)
	}
	passTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:         "Pass Type ID: " + passTypeIdentifier,
			OrganizationalUnit: []string{teamIdentifier},
			ExtraNames: []pkix.AttributeTypeAndValue{
				{Type: subjectUIDOID, Value: passTypeIdentifier},
			},
		},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	passDER, err := x509.CreateCertificate(rand.Reader, passTemplate, wwdrTemplate, &passKey.PublicKey, wwdrKey)
	if err != nil {
		t.Fatalf("create pass certificate: %v", err)
	}

	directory := t.TempDir()
	certificatePath := filepath.Join(directory, "pass-cert.pem")
	keyPath := filepath.Join(directory, "pass-key.pem")
	wwdrPath := filepath.Join(directory, "wwdr.pem")
	writePEMFile(t, certificatePath, "CERTIFICATE", passDER, 0o644)
	writePEMFile(t, keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(passKey), 0o600)
	writePEMFile(t, wwdrPath, "CERTIFICATE", wwdrDER, 0o644)
	return certificatePath, keyPath, wwdrPath
}

func writePEMFile(t *testing.T, path string, blockType string, der []byte, mode os.FileMode) {
	t.Helper()
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestPNG(t *testing.T, path string, width int, height int) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.White)
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encode %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

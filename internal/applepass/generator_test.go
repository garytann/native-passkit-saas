package applepass

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePassFilesUsesCurrentWalletFallbackIconSizes(t *testing.T) {
	directory := t.TempDir()
	if err := writePassFiles(directory, Pass{FormatVersion: 1}, "", nil); err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string]int{
		"icon.png":    38,
		"icon@2x.png": 76,
		"icon@3x.png": 114,
	} {
		file, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(file)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read %s: decode=%v close=%v", name, err, closeErr)
		}
		if cfg.Width != expected || cfg.Height != expected {
			t.Fatalf("%s dimensions = %dx%d, want %dx%d", name, cfg.Width, cfg.Height, expected, expected)
		}
	}
}

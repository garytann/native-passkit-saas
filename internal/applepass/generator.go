package applepass

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Generator struct {
	SignerCertPath  string
	SignerKeyPath   string
	WWDRCertPath    string
	OpenSSLBin      string
	AssetBundlePath string
	DynamicAssets   map[string][]byte
}

func (g Generator) Create(pass Pass) ([]byte, error) {
	if err := g.validate(); err != nil {
		return nil, err
	}

	workDir, err := os.MkdirTemp("", "apple-pass-*")
	if err != nil {
		return nil, fmt.Errorf("create temp pass directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := writePassFiles(workDir, pass, g.AssetBundlePath, g.DynamicAssets); err != nil {
		return nil, err
	}

	manifest, err := buildManifest(workDir)
	if err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(workDir, "manifest.json"), manifest); err != nil {
		return nil, err
	}

	if err := g.signManifest(workDir); err != nil {
		return nil, err
	}

	archive, err := zipDirectory(workDir)
	if err != nil {
		return nil, err
	}

	return archive, nil
}

func (g Generator) validate() error {
	if g.OpenSSLBin == "" {
		g.OpenSSLBin = "openssl"
	}
	for label, path := range map[string]string{
		"pass certificate": g.SignerCertPath,
		"pass private key": g.SignerKeyPath,
		"WWDR certificate": g.WWDRCertPath,
	} {
		if path == "" {
			return fmt.Errorf("%s path is required", label)
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%s not readable at %q: %w", label, path, err)
		}
	}
	if g.AssetBundlePath != "" {
		info, err := os.Stat(g.AssetBundlePath)
		if err != nil {
			return fmt.Errorf("pass asset bundle not readable at %q: %w", g.AssetBundlePath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("pass asset bundle %q is not a directory", g.AssetBundlePath)
		}
	}
	return nil
}

func writePassFiles(dir string, pass Pass, assetBundlePath string, dynamicAssets map[string][]byte) error {
	if err := writeJSON(filepath.Join(dir, "pass.json"), pass); err != nil {
		return err
	}
	if assetBundlePath != "" {
		if err := copyPassAssets(dir, assetBundlePath); err != nil {
			return err
		}
	} else {
		assets := map[string]imageSpec{
			"icon.png":    {38, 38, color.RGBA{84, 52, 245, 255}},
			"icon@2x.png": {76, 76, color.RGBA{84, 52, 245, 255}},
			"icon@3x.png": {114, 114, color.RGBA{84, 52, 245, 255}},
			"logo.png":    {160, 50, color.RGBA{255, 255, 255, 255}},
			"logo@2x.png": {320, 100, color.RGBA{255, 255, 255, 255}},
			"logo@3x.png": {480, 150, color.RGBA{255, 255, 255, 255}},
		}

		for name, spec := range assets {
			if err := writeSolidPNG(filepath.Join(dir, name), spec); err != nil {
				return err
			}
		}
	}
	for name, data := range dynamicAssets {
		if name != "strip.png" && name != "strip@2x.png" && name != "strip@3x.png" {
			return fmt.Errorf("unsupported dynamic pass asset %s", name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return fmt.Errorf("write dynamic pass asset %s: %w", name, err)
		}
	}

	return nil
}

func copyPassAssets(destination string, source string) error {
	for _, name := range []string{"icon.png", "icon@2x.png", "icon@3x.png", "logo.png", "logo@2x.png", "logo@3x.png"} {
		input, err := os.Open(filepath.Join(source, name))
		if err != nil {
			return fmt.Errorf("open pass asset %s: %w", name, err)
		}
		output, err := os.Create(filepath.Join(destination, name))
		if err != nil {
			_ = input.Close()
			return fmt.Errorf("create pass asset %s: %w", name, err)
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return fmt.Errorf("copy pass asset %s: %w", name, copyErr)
		}
		if inputCloseErr != nil || outputCloseErr != nil {
			return fmt.Errorf("close pass asset %s", name)
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

type imageSpec struct {
	width  int
	height int
	fill   color.RGBA
}

func writeSolidPNG(path string, spec imageSpec) error {
	img := image.NewRGBA(image.Rect(0, 0, spec.width, spec.height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: spec.fill}, image.Point{}, draw.Src)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func buildManifest(dir string) (map[string]string, error) {
	manifest := map[string]string{}

	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "manifest.json" || relative == "signature" || strings.HasPrefix(filepath.Base(relative), ".") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha1.Sum(data)
		manifest[relative] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("build manifest: %w", err)
	}

	return manifest, nil
}

func (g Generator) signManifest(dir string) error {
	signaturePath := filepath.Join(dir, "signature")
	manifestPath := filepath.Join(dir, "manifest.json")

	args := []string{
		"smime",
		"-binary",
		"-sign",
		"-signer", g.SignerCertPath,
		"-inkey", g.SignerKeyPath,
		"-certfile", g.WWDRCertPath,
		"-in", manifestPath,
		"-out", signaturePath,
		"-outform", "DER",
	}

	cmd := exec.Command(g.OpenSSLBin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sign manifest with openssl: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func zipDirectory(dir string) ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)

	paths := make([]string, 0)
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk pass directory: %w", err)
	}
	sort.Strings(paths)

	for _, path := range paths {
		if err := addZipFile(zipWriter, dir, path); err != nil {
			zipWriter.Close()
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buffer.Bytes(), nil
}

func addZipFile(zipWriter *zip.Writer, root string, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = relative
	header.Method = zip.Deflate
	header.Modified = time.Now().UTC()

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", relative, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", relative, err)
	}
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("write zip entry %s: %w", relative, err)
	}
	return nil
}

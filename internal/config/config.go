package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	PublicBaseURL         string
	AuthTokenSecret       string
	AdminAPIToken         string
	PlatformAdminEmail    string
	PlatformAdminPassword string
	CertificateBasePath   string
	PassAssetBasePath     string
	PassTypeID            string
	TeamID                string
	OrganizationName      string
	WebServiceURL         string
	PassCertPath          string
	PassKeyPath           string
	WWDRCertPath          string
	APNSEndpoint          string
	OpenSSLBin            string
}

func Load() Config {
	_ = loadDotEnv(".env")

	return Config{
		Port:                  env("PORT", "8080"),
		DatabaseURL:           env("DATABASE_URL", ""),
		PublicBaseURL:         env("PUBLIC_BASE_URL", "http://localhost:8080"),
		AuthTokenSecret:       env("AUTH_TOKEN_SECRET", "dev-only-change-me"),
		AdminAPIToken:         env("ADMIN_API_TOKEN", ""),
		PlatformAdminEmail:    env("PLATFORM_ADMIN_EMAIL", ""),
		PlatformAdminPassword: env("PLATFORM_ADMIN_PASSWORD", ""),
		CertificateBasePath:   env("APPLE_CERTIFICATE_BASE_PATH", "certs"),
		PassAssetBasePath:     env("APPLE_PASS_ASSET_BASE_PATH", "pass-assets"),
		PassTypeID:            env("APPLE_PASS_TYPE_ID", "pass.com.example.membership"),
		TeamID:                env("APPLE_TEAM_ID", "ABCDE12345"),
		OrganizationName:      env("APPLE_ORGANIZATION_NAME", "Example Company"),
		WebServiceURL:         env("APPLE_WEB_SERVICE_URL", ""),
		PassCertPath:          env("APPLE_PASS_CERT_PATH", "certs/pass-cert.pem"),
		PassKeyPath:           env("APPLE_PASS_KEY_PATH", "certs/pass-key.pem"),
		WWDRCertPath:          env("APPLE_WWDR_CERT_PATH", "certs/wwdr.pem"),
		APNSEndpoint:          env("APPLE_APNS_ENDPOINT", "https://api.push.apple.com"),
		OpenSSLBin:            env("OPENSSL_BIN", "openssl"),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

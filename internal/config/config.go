package config

import (
	"os"
)

type Config struct {
	Port             string
	PublicBaseURL    string
	PassTypeID       string
	TeamID           string
	OrganizationName string
	WebServiceURL    string
	PassCertPath     string
	PassKeyPath      string
	WWDRCertPath     string
	OpenSSLBin       string
}

func Load() Config {
	return Config{
		Port:             env("PORT", "8080"),
		PublicBaseURL:    env("PUBLIC_BASE_URL", "http://localhost:8080"),
		PassTypeID:       env("APPLE_PASS_TYPE_ID", "pass.com.example.membership"),
		TeamID:           env("APPLE_TEAM_ID", "ABCDE12345"),
		OrganizationName: env("APPLE_ORGANIZATION_NAME", "Example Company"),
		WebServiceURL:    env("APPLE_WEB_SERVICE_URL", ""),
		PassCertPath:     env("APPLE_PASS_CERT_PATH", "certs/pass-cert.pem"),
		PassKeyPath:      env("APPLE_PASS_KEY_PATH", "certs/pass-key.pem"),
		WWDRCertPath:     env("APPLE_WWDR_CERT_PATH", "certs/wwdr.pem"),
		OpenSSLBin:       env("OPENSSL_BIN", "openssl"),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

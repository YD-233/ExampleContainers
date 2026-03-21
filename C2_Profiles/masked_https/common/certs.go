package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EnsureTLSFiles(baseDir string, config RuntimeConfig) (RuntimeConfig, error) {
	config = normalizeConfig(config)
	if !config.UseSSL {
		return config, nil
	}

	certPath := resolveConfigFile(baseDir, config.CertFile, "masked_https.crt")
	keyPath := resolveConfigFile(baseDir, config.KeyFile, "masked_https.key")
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			config.CertFile = certPath
			config.KeyFile = keyPath
			return config, nil
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return config, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return config, err
	}

	host := config.HostHeader
	if strings.TrimSpace(host) == "" {
		host = strings.TrimPrefix(strings.TrimPrefix(config.CallbackHost, "https://"), "http://")
		host = strings.Split(host, "/")[0]
		host = strings.Split(host, ":")[0]
	}
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   host,
			Organization: []string{"masked_https"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return config, err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return config, err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return config, err
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return config, err
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
		return config, err
	}

	config.CertFile = certPath
	config.KeyFile = keyPath
	return config, nil
}

func resolveConfigFile(baseDir string, configured string, fallback string) string {
	trimmed := strings.TrimSpace(configured)
	if trimmed == "" {
		return filepath.Join(baseDir, fallback)
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(baseDir, trimmed)
}

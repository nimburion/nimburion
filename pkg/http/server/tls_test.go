package server

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTLSConfig(t *testing.T) {
	t.Run("missing files", func(t *testing.T) {
		_, err := LoadTLSConfig("/missing/server.crt", "/missing/server.key", "/missing/ca.crt")
		if err == nil {
			t.Fatal("expected error for missing certificate files")
		}
	})

	t.Run("invalid CA", func(t *testing.T) {
		dir := t.TempDir()
		_, serverCert, serverKey, _, _ := writeTestCertificates(t, dir)
		caPath := filepath.Join(dir, "invalid-ca.crt")
		if err := os.WriteFile(caPath, []byte("not-a-pem"), 0o644); err != nil {
			t.Fatalf("failed to write invalid CA file: %v", err)
		}

		_, err := LoadTLSConfig(serverCert, serverKey, caPath)
		if err == nil {
			t.Fatal("expected error for invalid CA file")
		}
	})

	t.Run("valid certificates", func(t *testing.T) {
		dir := t.TempDir()
		caPath, serverCert, serverKey, _, _ := writeTestCertificates(t, dir)

		cfg, err := LoadTLSConfig(serverCert, serverKey, caPath)
		if err != nil {
			t.Fatalf("expected valid TLS config, got error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil TLS config")
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Fatalf("expected TLS min version 1.2, got %d", cfg.MinVersion)
		}
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Fatalf("expected client auth RequireAndVerifyClientCert, got %v", cfg.ClientAuth)
		}
		if len(cfg.Certificates) != 1 {
			t.Fatalf("expected one server certificate, got %d", len(cfg.Certificates))
		}
		if cfg.ClientCAs == nil {
			t.Fatal("expected non-nil client CA pool")
		}
	})
}

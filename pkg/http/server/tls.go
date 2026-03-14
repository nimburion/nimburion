package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/nimburion/nimburion/internal/safepath"
)

// LoadServerTLSConfig creates a server-side TLS configuration without client certificate enforcement.
func LoadServerTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	if validateErr := safepath.ValidateFilePath(certFile, ""); validateErr != nil {
		return nil, fmt.Errorf("invalid server certificate path: %w", validateErr)
	}
	if validateErr := safepath.ValidateFilePath(keyFile, ""); validateErr != nil {
		return nil, fmt.Errorf("invalid server key path: %w", validateErr)
	}
	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate/key: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// LoadTLSConfig creates an mTLS-ready TLS configuration from certificate files.
func LoadTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	baseConfig, err := LoadServerTLSConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	if validateErr := safepath.ValidateFilePath(caFile, ""); validateErr != nil {
		return nil, fmt.Errorf("invalid CA certificate path: %w", validateErr)
	}
	// #nosec G304 -- caFile is validated before being read.
	caBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	clientCAPool := x509.NewCertPool()
	if !clientCAPool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("failed to parse CA certificate %s", caFile)
	}

	baseConfig.ClientAuth = tls.RequireAndVerifyClientCert
	baseConfig.ClientCAs = clientCAPool
	return baseConfig, nil
}

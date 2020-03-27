package golang

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

// generateTLSConfig generates a new TLS config based on given
// certificate path and key path.
func generateTLSConfig(certPath, keyPath, caCertPath string) (*tls.Config, error) {
	// Load certificate
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	// Create certificate pool
	certPool := x509.NewCertPool()
	rootCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}

	// Append cert to cert pool
	if ok := certPool.AppendCertsFromPEM(rootCert); !ok {
		return nil, errCertNotAppended
	}

	return &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	}, nil
}

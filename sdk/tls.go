package clusdr

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	caFile   = "ca.crt"
	certFile = "node.crt"
	keyFile  = "node.key"
)

func dialAddr(addr string, creds credentials.TransportCredentials) (*grpc.ClientConn, error) {
	if creds == nil {
		creds = insecure.NewCredentials()
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
}

func pemFilesPresent(dir string) bool {
	for _, name := range []string{caFile, certFile, keyFile} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !st.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func clientFromDir(dir string) (credentials.TransportCredentials, error) {
	ca, err := os.ReadFile(filepath.Join(dir, caFile))
	if err != nil {
		return nil, err
	}
	certPEM, err := os.ReadFile(filepath.Join(dir, certFile))
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, err
	}
	return clientTLS(ca, certPEM, keyPEM)
}

func clientTLS(caPEM, certPEM, keyPEM []byte) (credentials.TransportCredentials, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("clusdr: invalid CA certificate")
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("clusdr: client keypair: %w", err)
	}
	return credentials.NewTLS(&tls.Config{
		MinVersion:             tls.VersionTLS12,
		Certificates:           []tls.Certificate{cert},
		RootCAs:                pool,
		InsecureSkipVerify:     true, //nolint:gosec // G402: identity is CA, not dial hostname
		SessionTicketsDisabled: true, // VerifyPeerCertificate is skipped on resumed sessions
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			return verifyPeer(pool, raw)
		},
	}), nil
}

func verifyPeer(roots *x509.CertPool, raw [][]byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("clusdr: no peer certificate")
	}
	peer, err := x509.ParseCertificate(raw[0])
	if err != nil {
		return fmt.Errorf("clusdr: parse peer cert: %w", err)
	}
	opts := x509.VerifyOptions{Roots: roots}
	if len(raw) > 1 {
		opts.Intermediates = x509.NewCertPool()
		for _, der := range raw[1:] {
			c, err := x509.ParseCertificate(der)
			if err != nil {
				return fmt.Errorf("clusdr: parse intermediate: %w", err)
			}
			opts.Intermediates.AddCert(c)
		}
	}
	if _, err := peer.Verify(opts); err != nil {
		return fmt.Errorf("clusdr: peer cert not signed by cluster CA: %w", err)
	}
	return nil
}

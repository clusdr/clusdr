// Package mtls builds gRPC transport credentials from cluster PKI.
//
// Server TLS uses VerifyClientCertIfGiven: a joining node may connect
// without a client cert (Join is token-authenticated). After join, nodes
// present CA-issued certificates. Peer certificates from a foreign CA are
// rejected at the handshake.
package mtls

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
	CAFile   = "ca.crt"
	CertFile = "node.crt"
	KeyFile  = "node.key"
)

// Dial returns a connection using creds. nil creds → insecure.
func Dial(addr string, creds credentials.TransportCredentials) (*grpc.ClientConn, error) {
	if creds == nil {
		creds = insecure.NewCredentials()
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
}

// ServerTLS builds a server tls.Config that authenticates the node and
// optionally verifies client certificates against the cluster CA.
func ServerTLS(caPEM, certPEM, keyPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("mtls: server keypair: %w", err)
	}
	pool, err := poolFromPEM(caPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	}, nil
}

// ClientTLS builds client credentials that present the node cert and verify
// the peer against the cluster CA (hostname/SAN is not required — identity
// is the certificate, not the dial address).
func ClientTLS(caPEM, certPEM, keyPEM []byte) (credentials.TransportCredentials, error) {
	cfg, err := clientConfig(caPEM, certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(cfg), nil
}

// ClientFromDir loads node material from dir and builds ClientTLS credentials.
func ClientFromDir(dir string) (credentials.TransportCredentials, error) {
	ca, cert, key, err := LoadFiles(dir)
	if err != nil {
		return nil, err
	}
	return ClientTLS(ca, cert, key)
}

// BootstrapTLS is used for the first Join RPC: TLS to a server whose CA the
// joiner does not yet trust, with no client certificate. Token auth covers
// this hop; subsequent RPCs use ClientTLS.
func BootstrapTLS() credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // G402: join bootstrap; token-authenticated
	})
}

// ReloadableServerTLS rebuilds the handshake config from load on every
// connection so a join-issued certificate is picked up without restart.
func ReloadableServerTLS(load func() (ca, cert, key []byte, err error)) (*tls.Config, error) {
	if load == nil {
		return nil, fmt.Errorf("mtls: nil certificate loader")
	}
	ca, cert, key, err := load()
	if err != nil {
		return nil, err
	}
	if _, err := ServerTLS(ca, cert, key); err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.VerifyClientCertIfGiven,
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			ca, cert, key, err := load()
			if err != nil {
				return nil, err
			}
			return ServerTLS(ca, cert, key)
		},
	}, nil
}

func clientConfig(caPEM, certPEM, keyPEM []byte) (*tls.Config, error) {
	pool, err := poolFromPEM(caPEM)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("mtls: client keypair: %w", err)
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		InsecureSkipVerify: true, //nolint:gosec // G402: identity is CA, not dial hostname
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			return verifyPeer(pool, raw)
		},
	}, nil
}

func verifyPeer(roots *x509.CertPool, raw [][]byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("mtls: no peer certificate")
	}
	peer, err := x509.ParseCertificate(raw[0])
	if err != nil {
		return fmt.Errorf("mtls: parse peer cert: %w", err)
	}
	opts := x509.VerifyOptions{Roots: roots}
	if len(raw) > 1 {
		opts.Intermediates = x509.NewCertPool()
		for _, der := range raw[1:] {
			c, err := x509.ParseCertificate(der)
			if err != nil {
				return fmt.Errorf("mtls: parse intermediate: %w", err)
			}
			opts.Intermediates.AddCert(c)
		}
	}
	if _, err := peer.Verify(opts); err != nil {
		return fmt.Errorf("mtls: peer cert not signed by cluster CA: %w", err)
	}
	return nil
}

func poolFromPEM(caPEM []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("mtls: invalid CA certificate")
	}
	return pool, nil
}

// WriteFiles writes CA and node material next to state.db for CLI/apps
// that cannot open BoltDB while the daemon holds it.
func WriteFiles(dir string, caCert, nodeCert, nodeKey []byte) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	if len(caCert) > 0 {
		if err := os.WriteFile(filepath.Join(dir, CAFile), caCert, 0o644); err != nil {
			return err
		}
	}
	if len(nodeCert) > 0 {
		if err := os.WriteFile(filepath.Join(dir, CertFile), nodeCert, 0o644); err != nil {
			return err
		}
	}
	if len(nodeKey) > 0 {
		if err := os.WriteFile(filepath.Join(dir, KeyFile), nodeKey, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// LoadFiles reads ca.crt / node.crt / node.key from dir.
func LoadFiles(dir string) (ca, cert, key []byte, err error) {
	ca, err = os.ReadFile(filepath.Join(dir, CAFile))
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err = os.ReadFile(filepath.Join(dir, CertFile))
	if err != nil {
		return nil, nil, nil, err
	}
	key, err = os.ReadFile(filepath.Join(dir, KeyFile))
	if err != nil {
		return nil, nil, nil, err
	}
	return ca, cert, key, nil
}

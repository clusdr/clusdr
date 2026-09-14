// Package pki generates the cluster CA, node certificates, and join tokens.
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

const (
	caValidity   = 10 * 365 * 24 * time.Hour
	nodeValidity = 365 * 24 * time.Hour
	tokenBytes   = 32
)

// Bundle is the cluster PKI material persisted in the store.
type Bundle struct {
	CACert        []byte // PEM
	CAKey         []byte // PEM (PKCS#8)
	NodeCert      []byte // PEM
	NodeKey       []byte // PEM (PKCS#8)
	JoinTokenHash []byte // SHA-256 of the plaintext token
}

// Generate creates a new cluster CA, issues a cert for the seed node, and
// returns a one-time join token. The plaintext token is not stored in Bundle;
// only its SHA-256 hash is. Callers must print the token once and discard it.
func Generate(clusterID, nodeID string) (Bundle, string, error) {
	if clusterID == "" || nodeID == "" {
		return Bundle{}, "", fmt.Errorf("pki: cluster id and node id are required")
	}

	caCert, caKey, caCertPEM, caKeyPEM, err := generateCA(clusterID)
	if err != nil {
		return Bundle{}, "", err
	}
	nodeCertPEM, nodeKeyPEM, err := issueNode(caCert, caKey, nodeID)
	if err != nil {
		return Bundle{}, "", err
	}
	token, tokenHash, err := newJoinToken()
	if err != nil {
		return Bundle{}, "", err
	}

	return Bundle{
		CACert:        caCertPEM,
		CAKey:         caKeyPEM,
		NodeCert:      nodeCertPEM,
		NodeKey:       nodeKeyPEM,
		JoinTokenHash: tokenHash,
	}, token, nil
}

// Fingerprint returns the SHA-256 of the certificate DER, prefixed with "sha256:".
func Fingerprint(certPEM []byte) (string, error) {
	cert, err := ParseCertificate(certPEM)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.Raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ParseCertificate decodes the first PEM CERTIFICATE block.
func ParseCertificate(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("pki: no certificate PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse certificate: %w", err)
	}
	return cert, nil
}

// HashToken returns the SHA-256 of the plaintext join token.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// CheckToken reports whether token hashes to wantHash using constant time
// compare. An empty wantHash means "no token configured" and returns true.
func CheckToken(wantHash []byte, token string) bool {
	if len(wantHash) == 0 {
		return true
	}
	got := HashToken(token)
	if len(got) != len(wantHash) {
		return false
	}
	return subtle.ConstantTimeCompare(wantHash, got) == 1
}

// IssueNodeCert signs a new node certificate with the cluster CA.
func IssueNodeCert(caCertPEM, caKeyPEM []byte, nodeID string) (certPEM, keyPEM []byte, err error) {
	if nodeID == "" {
		return nil, nil, fmt.Errorf("pki: node id is required")
	}
	ca, err := ParseCertificate(caCertPEM)
	if err != nil {
		return nil, nil, err
	}
	caKey, err := parseCAKey(caKeyPEM)
	if err != nil {
		return nil, nil, err
	}
	return issueNode(ca, caKey, nodeID)
}

func parseCAKey(keyPEM []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("pki: no private key PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse ca key: %w", err)
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("pki: ca key is not ECDSA")
	}
	return ec, nil
}

func generateCA(clusterID string) (*x509.Certificate, *ecdsa.PrivateKey, []byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pki: generate ca key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"clusdr"},
			CommonName:   "clusdr-ca-" + clusterID,
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pki: create ca cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pki: parse ca cert: %w", err)
	}
	certPEM, keyPEM, err := encodePEM(der, key)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return cert, key, certPEM, keyPEM, nil
}

func issueNode(ca *x509.Certificate, caKey *ecdsa.PrivateKey, nodeID string) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("pki: generate node key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"clusdr"},
			CommonName:   nodeID,
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(nodeValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{nodeID},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("pki: create node cert: %w", err)
	}
	return encodePEM(der, key)
}

func encodePEM(certDER []byte, key *ecdsa.PrivateKey) (certPEM, keyPEM []byte, err error) {
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("pki: marshal key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	return certPEM, keyPEM, nil
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("pki: serial: %w", err)
	}
	return serial, nil
}

func newJoinToken() (string, []byte, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("pki: join token: %w", err)
	}
	token := hex.EncodeToString(b)
	return token, HashToken(token), nil
}

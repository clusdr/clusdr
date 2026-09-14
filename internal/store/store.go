// Package store manages persistent local state for the Clusdr daemon.
//
// Storage backend: bbolt (BoltDB). A single file holds all buckets:
//
//	<data_dir>/state.db
//	  bucket "node"    — node.id, cluster.id
//	  bucket "raft"    — reserved metadata (the Raft log is raft-boltdb)
//	  bucket "certs"   — CA cert/key, node cert/key, join token hash
//
// The Raft log itself is managed by hashicorp/raft-boltdb;
// this package owns the "node" bucket and overall file lifecycle.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFileName    = "state.db"
	dbFileMode    = 0o600
	dbOpenTimeout = 5 * time.Second
	// CLI inspect while the daemon may hold the exclusive flock.
	dbBriefTimeout = 200 * time.Millisecond

	bucketNode  = "node"
	bucketRaft  = "raft" // reserved
	bucketCerts = "certs"

	keyNodeID    = "node.id"
	keyClusterID = "cluster.id"

	keyCACert        = "ca.crt"
	keyCAKey         = "ca.key"
	keyNodeCert      = "node.crt"
	keyNodeKey       = "node.key"
	keyJoinTokenHash = "join_token.sha256"
)

var (
	// ErrNotFound is returned when a key is not present in the store.
	ErrNotFound = errors.New("not found")
	// ErrNotInitialized is returned when the store has no node identity set.
	// Run 'clusdr init' to fix this.
	ErrNotInitialized = errors.New("node not initialized: run 'clusdr init'")
)

// Store is the persistent local state for this node.
// It is safe for concurrent use; bbolt serializes writes internally.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the state database in dataDir.
// The caller must call Close when done.
func Open(dataDir string) (*Store, error) {
	return open(dataDir, dbOpenTimeout)
}

// OpenBrief is Open with a short lock wait. CLI inspect uses this so a
// running daemon does not block for dbOpenTimeout.
func OpenBrief(dataDir string) (*Store, error) {
	return open(dataDir, dbBriefTimeout)
}

func open(dataDir string, timeout time.Duration) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}

	path := filepath.Join(dataDir, dbFileName)
	db, err := bolt.Open(path, dbFileMode, &bolt.Options{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}

	s := &Store{db: db}
	if err := s.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database file.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SaveIdentity persists node and cluster IDs. Called once by 'clusdr init'.
// Overwrites any existing values.
func (s *Store) SaveIdentity(nodeID, clusterID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNode))
		if err := b.Put([]byte(keyNodeID), []byte(nodeID)); err != nil {
			return fmt.Errorf("put node.id: %w", err)
		}
		if err := b.Put([]byte(keyClusterID), []byte(clusterID)); err != nil {
			return fmt.Errorf("put cluster.id: %w", err)
		}
		return nil
	})
}

// Identity returns the persisted node and cluster IDs.
// Returns ErrNotInitialized if 'clusdr init' has not been run.
func (s *Store) Identity() (nodeID, clusterID string, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNode))
		nid := b.Get([]byte(keyNodeID))
		cid := b.Get([]byte(keyClusterID))
		if len(nid) == 0 || len(cid) == 0 {
			return ErrNotInitialized
		}
		nodeID = string(nid)
		clusterID = string(cid)
		return nil
	})
	return
}

// Certs is the PEM-encoded cluster PKI material stored in the certs bucket.
type Certs struct {
	CACert        []byte
	CAKey         []byte
	NodeCert      []byte
	NodeKey       []byte
	JoinTokenHash []byte
}

// SaveCerts persists CA and node material. Overwrites existing values.
func (s *Store) SaveCerts(c Certs) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketCerts))
		pairs := []struct {
			k string
			v []byte
		}{
			{keyCACert, c.CACert},
			{keyCAKey, c.CAKey},
			{keyNodeCert, c.NodeCert},
			{keyNodeKey, c.NodeKey},
			{keyJoinTokenHash, c.JoinTokenHash},
		}
		for _, p := range pairs {
			if len(p.v) == 0 {
				continue
			}
			if err := b.Put([]byte(p.k), p.v); err != nil {
				return fmt.Errorf("put %s: %w", p.k, err)
			}
		}
		return nil
	})
}

// SaveJoinCerts writes the cluster CA cert and an issued node certificate
// after a successful join. The CA private key is removed so this node cannot
// mint additional cluster certificates.
func (s *Store) SaveJoinCerts(caCert, nodeCert, nodeKey, tokenHash []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketCerts))
		pairs := []struct {
			k string
			v []byte
		}{
			{keyCACert, caCert},
			{keyNodeCert, nodeCert},
			{keyNodeKey, nodeKey},
			{keyJoinTokenHash, tokenHash},
		}
		for _, p := range pairs {
			if len(p.v) == 0 {
				continue
			}
			if err := b.Put([]byte(p.k), p.v); err != nil {
				return fmt.Errorf("put %s: %w", p.k, err)
			}
		}
		if err := b.Delete([]byte(keyCAKey)); err != nil {
			return fmt.Errorf("delete ca.key: %w", err)
		}
		return nil
	})
}

// Certs loads PKI material. Returns ErrNotInitialized if the CA is missing.
func (s *Store) Certs() (Certs, error) {
	var c Certs
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketCerts))
		c.CACert = clone(b.Get([]byte(keyCACert)))
		c.CAKey = clone(b.Get([]byte(keyCAKey)))
		c.NodeCert = clone(b.Get([]byte(keyNodeCert)))
		c.NodeKey = clone(b.Get([]byte(keyNodeKey)))
		c.JoinTokenHash = clone(b.Get([]byte(keyJoinTokenHash)))
		if len(c.CACert) == 0 {
			return ErrNotInitialized
		}
		return nil
	})
	return c, err
}

func clone(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// initBuckets creates all top-level buckets if they do not exist.
func (s *Store) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{bucketNode, bucketRaft, bucketCerts} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

package app

import (
	"fmt"

	"github.com/clusdr/clusdr/internal/mtls"
	"github.com/clusdr/clusdr/internal/pki"
	"github.com/clusdr/clusdr/internal/store"
)

// joinSec implements grpcserver.JoinSecurity using the local store.
type joinSec struct {
	store *store.Store
}

func (j *joinSec) CheckToken(token string) error {
	c, err := j.store.Certs()
	if err != nil || len(c.JoinTokenHash) == 0 {
		return nil
	}
	if !pki.CheckToken(c.JoinTokenHash, token) {
		return fmt.Errorf("unauthorized")
	}
	return nil
}

func (j *joinSec) IssueNode(nodeID string) (nodeCert, nodeKey, caCert, tokenHash []byte, err error) {
	c, err := j.store.Certs()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(c.CAKey) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("this node has no CA key; join via a seed node")
	}
	nodeCert, nodeKey, err = pki.IssueNodeCert(c.CACert, c.CAKey, nodeID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return nodeCert, nodeKey, c.CACert, c.JoinTokenHash, nil
}

// joinBoot implements grpcserver.JoinBootstrap.
type joinBoot struct {
	store   *store.Store
	dataDir string
}

func (j *joinBoot) SaveJoinCerts(caCert, nodeCert, nodeKey, tokenHash []byte) error {
	if err := j.store.SaveJoinCerts(caCert, nodeCert, nodeKey, tokenHash); err != nil {
		return err
	}
	if j.dataDir == "" {
		return nil
	}
	return mtls.WriteFiles(j.dataDir, caCert, nodeCert, nodeKey)
}

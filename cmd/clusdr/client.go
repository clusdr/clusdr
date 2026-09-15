package main

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/durguto/clusdr/internal/config"
	"github.com/durguto/clusdr/internal/mtls"
)

// dialDaemon opens a gRPC connection to the running daemon's Runtime API.
// The caller is responsible for calling conn.Close().
func dialDaemon(cfg config.Config) (*grpc.ClientConn, error) {
	conn, err := mtls.Dial(cfg.GRPC.Addr, daemonCreds(cfg))
	if err != nil {
		return nil, fmt.Errorf("dial daemon at %s: %w", cfg.GRPC.Addr, err)
	}
	return conn, nil
}

func daemonCreds(cfg config.Config) credentials.TransportCredentials {
	if !cfg.TLSEnabled() {
		return nil
	}
	creds, err := mtls.ClientFromDir(cfg.Data.Dir)
	if err != nil {
		// TLS is on but node cert files are missing: speak TLS without a
		// client certificate so Health (and Join bootstrap) still work.
		return mtls.BootstrapTLS()
	}
	return creds
}

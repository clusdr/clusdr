package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	bolt "go.etcd.io/bbolt"

	"github.com/odurgut/clusdr/internal/mtls"
	"github.com/odurgut/clusdr/internal/pki"
	"github.com/odurgut/clusdr/internal/store"
)

func newCertsCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Manage cluster certificates",
		Long:  "Inspect the cluster CA and node certificates stored on this node.",
	}
	cmd.AddCommand(newCertsShowCmd(f))
	return cmd
}

func newCertsShowCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show cluster CA and node certificate details",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}
			return showCerts(cmd.OutOrStdout(), cfg.Data.Dir)
		},
	}
}

func showCerts(out io.Writer, dataDir string) error {
	caPEM, err := os.ReadFile(filepath.Join(dataDir, mtls.CAFile))
	if err != nil {
		return fmt.Errorf("read %s: %w", mtls.CAFile, err)
	}

	ca, err := pki.ParseCertificate(caPEM)
	if err != nil {
		return fmt.Errorf("parse CA: %w", err)
	}
	fp, err := pki.Fingerprint(caPEM)
	if err != nil {
		return fmt.Errorf("fingerprint: %w", err)
	}

	fmt.Fprintln(out, "cluster CA")
	fmt.Fprintf(out, "  subject      : %s\n", ca.Subject.CommonName)
	fmt.Fprintf(out, "  fingerprint  : %s\n", fp)
	fmt.Fprintf(out, "  not before   : %s\n", ca.NotBefore.UTC().Format(time.RFC3339))
	fmt.Fprintf(out, "  not after    : %s\n", ca.NotAfter.UTC().Format(time.RFC3339))
	fmt.Fprintf(out, "  ca.crt       : %s\n", filepath.Join(dataDir, mtls.CAFile))

	nodePEM, err := os.ReadFile(filepath.Join(dataDir, mtls.CertFile))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", mtls.CertFile, err)
	}
	if len(nodePEM) > 0 {
		node, err := pki.ParseCertificate(nodePEM)
		if err != nil {
			return fmt.Errorf("parse node cert: %w", err)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "node certificate")
		fmt.Fprintf(out, "  subject      : %s\n", node.Subject.CommonName)
		fmt.Fprintf(out, "  not before   : %s\n", node.NotBefore.UTC().Format(time.RFC3339))
		fmt.Fprintf(out, "  not after    : %s\n", node.NotAfter.UTC().Format(time.RFC3339))
	}

	fmt.Fprintln(out)
	printJoinToken(out, dataDir)
	return nil
}

func printJoinToken(out io.Writer, dataDir string) {
	st, err := store.OpenBrief(dataDir)
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			fmt.Fprintln(out, "join token     : (store locked)")
			return
		}
		fmt.Fprintf(out, "join token     : unavailable (%v)\n", err)
		return
	}
	defer st.Close()

	c, err := st.Certs()
	if err != nil {
		if errors.Is(err, store.ErrNotInitialized) {
			fmt.Fprintln(out, "join token     : not set")
			return
		}
		fmt.Fprintf(out, "join token     : unavailable (%v)\n", err)
		return
	}

	if len(c.JoinTokenHash) == 0 {
		fmt.Fprintln(out, "join token     : not set")
		return
	}
	prefix := hex.EncodeToString(c.JoinTokenHash)
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	fmt.Fprintf(out, "join token     : configured (sha256:%s…)\n", prefix)
	fmt.Fprintln(out, "                plaintext is shown only at 'clusdr init'")
}

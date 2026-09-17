package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/clusdr/clusdr/internal/operator"
	"github.com/clusdr/clusdr/internal/version"
)

func main() {
	if versionRequested(os.Args[1:]) {
		fmt.Printf("clusdr-operator %s (commit: %s, built: %s)\n",
			version.Version, version.Commit, version.BuildTime)
		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("clusdr-operator", "version", version.Version, "commit", version.Commit)

	cfg, err := kubeConfig()
	if err != nil {
		slog.Error("kube config", "err", err)
		os.Exit(1)
	}
	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		slog.Error("kubernetes client", "err", err)
		os.Exit(1)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		slog.Error("dynamic client", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ns := os.Getenv("WATCH_NAMESPACE")
	if err := operator.Run(ctx, core, dyn, ns, operator.Runtime{}); err != nil {
		slog.Error("run", "err", err)
		os.Exit(1)
	}
}

func kubeConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}

func versionRequested(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "version", "--version", "-version":
		return true
	default:
		return false
	}
}

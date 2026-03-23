package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"kube-watcher/watcher/deploy"
)

func main() {
	cfg := parseFlags()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := deploy.Run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseFlags() deploy.Config {
	cfg := deploy.DefaultConfig()
	flag.StringVar(&cfg.Action, "action", cfg.Action, "action to execute: deploy|cleanup|status|logs|print-manifest")
	flag.StringVar(&cfg.Target, "target", cfg.Target, "runtime target: kube|docker|compose")
	flag.StringVar(&cfg.Image, "image", cfg.Image, "container image for kube-anomaly-detection or watcher")
	flag.StringVar(&cfg.SourcePath, "source-path", cfg.SourcePath, "optional local path to build image before deploy")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "application name / deployment name (defaults to kube-anomaly-detection-<cluster-name> or watcher-<cluster-name>)")
	flag.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "cluster identifier (required)")
	flag.StringVar(&cfg.Namespace, "namespace", cfg.Namespace, "kubernetes namespace")
	flag.BoolVar(&cfg.CreateNamespace, "create-namespace", cfg.CreateNamespace, "create namespace when deploying to kubernetes")
	flag.BoolVar(&cfg.DeleteNamespace, "delete-namespace", cfg.DeleteNamespace, "delete namespace on cleanup (kubernetes only)")
	flag.StringVar(&cfg.PrometheusEndpoint, "prometheus-endpoint", cfg.PrometheusEndpoint, "prometheus scrape endpoint (full /metrics URL, required for deploy)")
	flag.StringVar(&cfg.GoogleChatWebhook, "google-chat-webhook-url", cfg.GoogleChatWebhook, "optional Google Chat webhook URL")
	flag.StringVar(&cfg.AlertWebhook, "alert-webhook-url", cfg.AlertWebhook, "optional generic alert webhook URL (preferred over google-chat-webhook-url)")
	flag.StringVar(&cfg.KubeconfigPath, "kubeconfig-path", cfg.KubeconfigPath, "path to kubeconfig file for docker target (mounts into container)")
	flag.StringVar(&cfg.DockerConfigPath, "docker-config-path", cfg.DockerConfigPath, "local config path for docker target")
	flag.StringVar(&cfg.DockerDataDir, "docker-data-dir", cfg.DockerDataDir, "docker volume name for DuckDB data")
	flag.StringVar(&cfg.PVCName, "pvc-name", cfg.PVCName, "kubernetes persistent volume claim name (defaults to <deployment-name>-data)")
	flag.StringVar(&cfg.PVCSize, "pvc-size", cfg.PVCSize, "kubernetes persistent volume claim size")
	flag.IntVar(&cfg.TailLines, "tail-lines", cfg.TailLines, "number of log lines to fetch for logs action")
	flag.BoolVar(&cfg.Follow, "follow", cfg.Follow, "stream logs for logs action")
	flag.StringVar(&cfg.AppType, "app-type", cfg.AppType, "application type: kad (kube-anomaly-detection) or watcher")
	flag.StringVar(&cfg.DashboardWebhook, "dashboard-webhook", cfg.DashboardWebhook, "dashboard webhook URL for watcher alerts")
	flag.StringVar(&cfg.ComposeFile, "compose-file", cfg.ComposeFile, "docker-compose file path for compose target")
	flag.StringVar(&cfg.ComposeService, "compose-service", cfg.ComposeService, "compose service name for logs (optional)")
	flag.StringVar(&cfg.ComposeProject, "compose-project", cfg.ComposeProject, "compose project name (optional)")
	flag.Parse()

	normalized, err := deploy.NormalizeAndValidate(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(2)
	}
	return normalized
}

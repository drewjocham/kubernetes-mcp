package cmd

func defaultConfigExample() string {
	return `# kube-watcher configuration
ops:
  image: "drewjocham/kube-watcher:latest"
  name_prefix: "kw"
  kubeconfig: "~/.kube/config"
  db_path: "~/.kube-watcher"
  watcher_config: "~/.kube-watcher/watcher.yaml"
  network_mode: "host"
  interval: "30s"
  build_if_missing: true
  kube_namespace: "kubewatcher"
  kube_storage_class: ""
  kube_history_storage: "1Gi"

view:
  output: "table"

agents:
  sre-bot:
    description: "Specialist in CrashLoopBackOff analysis"
    role: "k8s-specialist"
    capabilities:
      - "get_pod_resources"
      - "analyze_cluster"
      - "list_repeating_issues"
    created_at: "2026-03-19"
`
}

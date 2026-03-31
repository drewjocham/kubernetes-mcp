# Test Configuration for Kube-Watcher

This configuration is for testing the Kube-Watcher desktop app with a Minikube cluster.

## Context
- **Cluster**: Minikube
- **Purpose**: Testing all functionality of Kube-Watcher app including Popeye integration, agent capabilities, and UI features.

## Test Setup
1. **Minikube Cluster**: Ensure Minikube is running (`minikube start`).
2. **Test Pods**: Create sample pods for data testing.
3. **Prometheus Instance**: Deploy a Prometheus instance for metrics collection and scanning.
4. **Configuration**: Use this file as a reference for test scenarios.

## Test Scenarios
- **UI Navigation**: Test all screens (Alerts, Anomalies, Charts, Logs, Hints, Popeye Dashboard, Deployments, Agent History, Config).
- **Popeye Scan**: Run cluster scan and verify dashboard output.
- **Agent Interaction**: Test autonomy levels, chat responses, action proposals, and approvals.
- **Data Visualization**: Verify charts and stats update with cluster data.
- **Service Management**: Start/stop services and verify status changes.

## Sample Pod Creation
```bash
kubectl run test-pod-1 --image=nginx --restart=Never
kubectl run test-pod-2 --image=busybox --restart=Never -- /bin/sh -c "sleep 3600"
kubectl run test-pod-3 --image=alpine --restart=Never -- /bin/sh -c "sleep 3600"
```

## Prometheus Deployment for Testing
```bash
# Create a namespace for Prometheus
kubectl create namespace prometheus

# Deploy Prometheus using Helm (if Helm is installed)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/prometheus -n prometheus --set server.persistentVolume.enabled=false

# Alternatively, deploy a simple Prometheus instance manually
cat <<EOF | kubectl apply -n prometheus -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/metrics
  - services
  - endpoints
  - pods
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources:
  - configmaps
  verbs: ["get"]
- apiGroups: ["networking.k8s.io"]
  resources:
  - ingresses
  verbs: ["get", "list", "watch"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: prometheus
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  labels:
    app: prometheus
spec:
  ports:
  - port: 9090
    targetPort: 9090
    protocol: TCP
  selector:
    app: prometheus
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  labels:
    app: prometheus
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      serviceAccountName: prometheus
      containers:
      - name: prometheus
        image: prom/prometheus:v2.43.0
        args:
        - "--config.file=/etc/prometheus/prometheus.yml"
        - "--storage.tsdb.path=/prometheus"
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: config-volume
          mountPath: /etc/prometheus
      volumes:
      - name: config-volume
        configMap:
          name: prometheus-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
      - job_name: 'kubernetes-apiservers'
        kubernetes_sd_configs:
          - role: endpoints
        scheme: https
        tls_config:
          ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
        relabel_configs:
          - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
            action: keep
            regex: default;kubernetes;https
      - job_name: 'kubernetes-nodes'
        scheme: https
        tls_config:
          ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
        kubernetes_sd_configs:
          - role: node
        relabel_configs:
          - action: labelmap
            regex: __meta_kubernetes_node_label_(.+)
      - job_name: 'kubernetes-pods'
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
            action: keep
            regex: true
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
            action: replace
            target_label: __metrics_path__
            regex: (.+)
          - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
            action: replace
            regex: ([^:]+)(?::\d+)?;(\d+)
            replacement: $1:$2
            target_label: __address__
          - action: labelmap
            regex: __meta_kubernetes_pod_label_(.+)
          - source_labels: [__meta_kubernetes_namespace]
            action: replace
            target_label: kubernetes_namespace
          - source_labels: [__meta_kubernetes_pod_name]
            action: replace
            target_label: kubernetes_pod_name
EOF

# Verify Prometheus deployment
kubectl get pods -n prometheus
kubectl get svc -n prometheus
```

## Accessing Prometheus
- Access Prometheus UI by running: `minikube service prometheus -n prometheus`
- Configure Kube-Watcher to use Prometheus endpoint (default: http://localhost:9090 if port-forwarded)

## Notes
- Ensure Minikube is the active context (`kubectl config use-context minikube`).
- MCP server should be running for full functionality (`make run-server`).

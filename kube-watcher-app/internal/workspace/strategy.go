package workspace

import "kube-watcher-app/internal/data"

// DeploymentStrategy swaps deployment guidance without branching in the handler.
type DeploymentStrategy interface {
	Plan() data.AnomalyDeploymentPlan
}

// LocalStrategy models a laptop-first Anomstack workflow.
type LocalStrategy struct{}

// Plan returns the local docker-compose path.
func (LocalStrategy) Plan() data.AnomalyDeploymentPlan {
	return data.AnomalyDeploymentPlan{
		Profile:   "local",
		Title:     "Local compose stack",
		Summary:   "Runs the full Anomstack control plane beside kube-watcher with the seeded kube metrics config.",
		Mode:      "Docker Compose",
		Namespace: "local",
		Services:  []string{"anomstack_webserver", "anomstack_daemon", "anomstack_dashboard"},
		Commands: []string{
			"kw anomstack start --path /path/to/anomstack",
			"kw anomstack status --path /path/to/anomstack",
		},
		Validation: []string{
			"curl http://localhost:3000/server_info",
			"curl http://localhost:5001/health",
		},
		Artifacts: []string{
			"deploy/anomaly/README.md",
			"anomstack-metrics/system_metrics/system_metrics.yaml",
		},
	}
}

// MinikubeStrategy models a single-node Kubernetes deployment path.
type MinikubeStrategy struct{}

// Plan returns the minikube deployment path.
func (MinikubeStrategy) Plan() data.AnomalyDeploymentPlan {
	return data.AnomalyDeploymentPlan{
		Profile:   "minikube",
		Title:     "Minikube single-node cluster",
		Summary:   "Builds local Anomstack images into the minikube Docker daemon and applies the embedded kube manifests plus mock feeder infra.",
		Mode:      "Kubernetes",
		Namespace: "kw-anomaly",
		Services:  []string{"anomstack-webserver", "anomstack-daemon", "anomstack-dashboard", "kw-anomaly-mock-feeder"},
		Commands: []string{
			"ANOMSTACK_PATH=/path/to/anomstack",
			"eval $(minikube docker-env)",
			"docker build -t anomstack_dagster_image:kw-local -f $ANOMSTACK_PATH/docker/Dockerfile.dagster $ANOMSTACK_PATH",
			"docker build -t anomstack_dashboard_image:kw-local -f $ANOMSTACK_PATH/docker/Dockerfile.anomstack_dashboard $ANOMSTACK_PATH",
			"kubectl apply -k deploy/anomaly/minikube",
		},
		Validation: []string{
			"kubectl get pods -n kw-anomaly",
			"kubectl port-forward -n kw-anomaly svc/anomstack-webserver 3000:3000",
			"kubectl port-forward -n kw-anomaly svc/anomstack-dashboard 5001:8080",
		},
		Artifacts: []string{
			"deploy/anomaly/minikube/kustomization.yaml",
			"deploy/anomaly/minikube/configmap-env.yaml",
			"deploy/anomaly/minikube/configmap-metrics.yaml",
			"deploy/anomaly/minikube/deploy-webserver.yaml",
			"deploy/anomaly/minikube/deploy-daemon.yaml",
			"deploy/anomaly/minikube/deploy-dashboard.yaml",
			"deploy/anomaly/minikube/mock-feeder.yaml",
		},
	}
}

// ClusterStrategy models a remote cluster deployment path.
type ClusterStrategy struct{}

// Plan returns the remote-cluster deployment path.
func (ClusterStrategy) Plan() data.AnomalyDeploymentPlan {
	return data.AnomalyDeploymentPlan{
		Profile:   "cluster",
		Title:     "Remote cluster rollout",
		Summary:   "Pushes versioned Anomstack images and applies the same control-plane shape with registry-backed tags and durable storage.",
		Mode:      "Kubernetes",
		Namespace: "kw-anomaly",
		Services:  []string{"anomstack-webserver", "anomstack-daemon", "anomstack-dashboard", "kw-anomaly-mock-feeder"},
		Commands: []string{
			"# Tag and push your images, e.g.",
			"docker tag anomstack_dagster_image:kw-local <registry>/anomstack_dagster_image:<tag>",
			"docker push <registry>/anomstack_dagster_image:<tag>",
			"docker tag anomstack_dashboard_image:kw-local <registry>/anomstack_dashboard_image:<tag>",
			"docker push <registry>/anomstack_dashboard_image:<tag>",
			"# Adapt deploy/anomaly/minikube manifests with your registry/tag and apply",
			"kubectl apply -k deploy/anomaly/minikube",
		},
		Validation: []string{
			"kubectl rollout status deploy/anomstack-webserver -n kw-anomaly",
			"kubectl rollout status deploy/anomstack-dashboard -n kw-anomaly",
		},
		Artifacts: []string{
			"deploy/anomaly/README.md",
			"deploy/anomaly/minikube/kustomization.yaml",
		},
	}
}

// DefaultDeploymentStrategies returns the supported anomaly deployment targets.
func DefaultDeploymentStrategies() []DeploymentStrategy {
	return []DeploymentStrategy{
		LocalStrategy{},
		MinikubeStrategy{},
		ClusterStrategy{},
	}
}

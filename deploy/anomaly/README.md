## Anomaly deployment tracks

- **Local compose**: use the upstream `anomstack` repo’s `docker-compose.yaml`; quick start when you stay on your laptop.
- **Minikube**: uses `deploy/anomaly/minikube` with local images tagged `anomstack_dagster_image:kw-local` and `anomstack_dashboard_image:kw-local`.
- **Cluster**: mirror the minikube kustomization and retag images in your registry; namespace defaults to `kw-anomaly`.

### Minikube quickstart

```bash
ANOMSTACK_PATH=/path/to/anomstack
eval $(minikube docker-env)
docker build -t anomstack_dagster_image:kw-local -f $ANOMSTACK_PATH/docker/Dockerfile.dagster $ANOMSTACK_PATH
docker build -t anomstack_dashboard_image:kw-local -f $ANOMSTACK_PATH/docker/Dockerfile.anomstack_dashboard $ANOMSTACK_PATH
kubectl apply -k deploy/anomaly/minikube
kubectl -n kw-anomaly port-forward svc/anomstack-webserver 3000:3000 &
kubectl -n kw-anomaly port-forward svc/anomstack-dashboard 5001:8080 &
```

### Local compose

```bash
ANOMSTACK_PATH=/path/to/anomstack
cd $ANOMSTACK_PATH
docker compose up -d
```

### Cluster (registry-based)

1. Push both images to your registry with tags `anomstack_dagster_image:<tag>` and `anomstack_dashboard_image:<tag>`.
2. Copy `deploy/anomaly/minikube` to a new folder, adjust the image tags and storage class, and apply with `kubectl apply -k`.

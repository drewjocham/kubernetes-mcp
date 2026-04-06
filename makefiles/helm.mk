# ── Variables ────────────────────────────────────────────────────────────────
HELM_CHART_DIR ?= "helm/kube-watcher"
HELM_VALUES    ?= "helm/kube-watcher/values.yaml"
HELM_RELEASE   ?= "local"
KW_NAMESPACE   ?= "default"

.PHONY: helm-lint helm-template helm-dry-run \
    helm-install helm-upgrade helm-uninstall \
    helm-status helm-diff helm-rollback

# ── Validation & Template ────────────────────────────────────────────────────

helm-lint:
	@echo "Linting $(HELM_CHART_DIR)..."
	helm lint $(HELM_CHART_DIR)

helm-template:
	@echo "Rendering templates..."
	helm template $(HELM_RELEASE) $(HELM_CHART_DIR) -f $(HELM_VALUES) --namespace $(KW_NAMESPACE)

helm-dry-run:
	@echo "Dry-run (server-side)..."
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
	   -f $(HELM_VALUES) \
	   --namespace $(KW_NAMESPACE) \
	   --dry-run --debug

# ── Release Lifecycle ────────────────────────────────────────────────────────

helm-install:
	@echo "Installing release: $(HELM_RELEASE) in namespace: $(KW_NAMESPACE)..."
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
	   -f $(HELM_VALUES) \
	   --namespace $(KW_NAMESPACE) \
	   --create-namespace \
	   --wait --timeout 5m

helm-upgrade:
	@echo "Upgrading release: $(HELM_RELEASE) (atomic — rolls back on failure)..."
	helm upgrade $(HELM_RELEASE) $(HELM_CHART_DIR) \
	   -f $(HELM_VALUES) \
	   --namespace $(KW_NAMESPACE) \
	   --wait --timeout 5m \
	   --atomic

helm-uninstall:
	@echo "Uninstalling release: $(HELM_RELEASE) from namespace: $(KW_NAMESPACE)..."
	helm uninstall $(HELM_RELEASE) --namespace $(KW_NAMESPACE)

# ── Release Inspection ───────────────────────────────────────────────────────

helm-status:
	@echo "Checking status of $(HELM_RELEASE)..."
	helm status $(HELM_RELEASE) --namespace $(KW_NAMESPACE)
	@echo "\nRevision History:"
	helm history $(HELM_RELEASE) --namespace $(KW_NAMESPACE)

helm-diff:
	@echo "Diffing pending changes..."
	@echo "Requires helm-diff plugin: helm plugin install https://github.com/databus23/helm-diff"
	helm diff upgrade $(HELM_RELEASE) $(HELM_CHART_DIR) -f $(HELM_VALUES) --namespace $(KW_NAMESPACE)

helm-rollback:
	@echo "Rolling back $(HELM_RELEASE) to previous revision..."
	helm rollback $(HELM_RELEASE) 0 --namespace $(KW_NAMESPACE) --wait

.PHONY: helm-lint helm-template helm-dry-run \
	helm-install helm-upgrade helm-uninstall \
	helm-status helm-diff helm-rollback

HELM_CHART_DIR="../helm/kube-watcher"
HELM_VALUES="../helm/kube-watcher/values.yaml"
HELM_RELEASE="local"


helm-lint:
	@echo "Linting $(HELM_CHART_DIR)..."
	helm lint $(HELM_CHART_DIR)

helm-template:
	@echo "Rendering templates..."
	helm template $(HELM_RELEASE) $(HELM_CHART_DIR) -f $(HELM_VALUES)

helm-dry-run:
	@echo "Dry-run (server-side)..."
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
		-f $(HELM_VALUES) \
		--dry-run --debug

# ── Release Lifecycle ──────────────────────────────────────────────────────────

helm-install:
	@echo "Installing release: $(HELM_RELEASE)..."
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
		-f $(HELM_VALUES) \
		--create-namespace \
		--wait --timeout 5m

helm-upgrade:
	@echo "Upgrading release: $(HELM_RELEASE) (atomic — rolls back on failure)..."
	helm upgrade $(HELM_RELEASE) $(HELM_CHART_DIR) \
		-f $(HELM_VALUES) \
		--wait --timeout 5m \
		--atomic

helm-uninstall:
	@echo "Uninstalling release: $(HELM_RELEASE)..."
	helm uninstall $(HELM_RELEASE) --namespace $(KW_NAMESPACE)

# ── Release Inspection ─────────────────────────────────────────────────────────

helm-status:
	helm status $(HELM_RELEASE) --namespace $(KW_NAMESPACE)
	@echo ""
	helm history $(HELM_RELEASE) --namespace $(KW_NAMESPACE)

helm-diff:
	@echo "Diffing pending changes (requires: helm plugin install https://github.com/databus23/helm-diff)..."
	helm diff upgrade $(HELM_RELEASE) $(HELM_CHART_DIR) -f $(HELM_VALUES)

helm-rollback:
	@echo "Rolling back $(HELM_RELEASE) to previous revision..."
	helm rollback $(HELM_RELEASE) 0 --namespace $(KW_NAMESPACE) --wait

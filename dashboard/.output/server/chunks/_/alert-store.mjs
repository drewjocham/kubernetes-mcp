import { EventEmitter } from 'node:events';

const STORE_KEY = "__kube_watcher_dashboard_store__";
function defaultConfig() {
  return {
    mcpEndpoint: "",
    mcpApiKeyHeader: "Authorization",
    mcpApiKey: "",
    agentEndpoint: "",
    agentApiKeyHeader: "Authorization",
    agentApiKey: "",
    watchedErrors: ["CrashLoopBackOff", "OOMKilled", "ImagePullBackOff"],
    autoApplyFixes: false
  };
}
function getStore() {
  const globalStore = globalThis;
  if (!globalStore[STORE_KEY]) {
    globalStore[STORE_KEY] = {
      alerts: [],
      config: defaultConfig(),
      bus: new EventEmitter()
    };
  }
  return globalStore[STORE_KEY];
}
function notify() {
  const store = getStore();
  store.bus.emit("alert-updated", store.alerts);
}
function normalizePayload(payload) {
  var _a, _b, _c;
  return {
    ...payload,
    source: (_a = payload.source) != null ? _a : "kube-watcher",
    severity: (_b = payload.severity) != null ? _b : "critical",
    firstSeenAt: (_c = payload.firstSeenAt) != null ? _c : (/* @__PURE__ */ new Date()).toISOString()
  };
}
function listAlerts() {
  return [...getStore().alerts].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}
function getAlert(id) {
  return getStore().alerts.find((alert) => alert.id === id);
}
function createAlert(payload) {
  const store = getStore();
  const now = (/* @__PURE__ */ new Date()).toISOString();
  const normalized = normalizePayload(payload);
  const alert = {
    ...normalized,
    id: crypto.randomUUID(),
    status: "detected",
    createdAt: now,
    updatedAt: now,
    thinkingSteps: []
  };
  store.alerts.unshift(alert);
  notify();
  return alert;
}
function updateAlert(id, mutator) {
  const store = getStore();
  const idx = store.alerts.findIndex((alert) => alert.id === id);
  if (idx < 0) return void 0;
  const current = store.alerts[idx];
  const updated = {
    ...mutator(current),
    updatedAt: (/* @__PURE__ */ new Date()).toISOString()
  };
  store.alerts[idx] = updated;
  notify();
  return updated;
}
function pushThinkingStep(id, step) {
  return updateAlert(id, (current) => ({
    ...current,
    thinkingSteps: [...current.thinkingSteps, step]
  }));
}
function setThinking(id) {
  return updateAlert(id, (current) => ({ ...current, status: "thinking" }));
}
function setReport(id, report) {
  return updateAlert(id, (current) => ({
    ...current,
    status: "report_ready",
    report
  }));
}
function setFailed(id, error) {
  return updateAlert(id, (current) => ({
    ...current,
    status: "failed",
    error
  }));
}
function getWorkflowConfig() {
  return getStore().config;
}
function setWorkflowConfig(next) {
  const store = getStore();
  store.config = {
    ...store.config,
    ...next
  };
  notify();
  return store.config;
}
function onStoreUpdates(handler) {
  const store = getStore();
  store.bus.on("alert-updated", handler);
  return () => store.bus.off("alert-updated", handler);
}

export { setReport as a, setFailed as b, getWorkflowConfig as c, createAlert as d, setWorkflowConfig as e, getAlert as g, listAlerts as l, onStoreUpdates as o, pushThinkingStep as p, setThinking as s, updateAlert as u };
//# sourceMappingURL=alert-store.mjs.map

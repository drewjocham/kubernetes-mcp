<template>
  <aside :class="['sidebar', { collapsed }]">
    <div class="sidebar-header">
      <h2>Argus Console</h2>
      <p class="sidebar-subtitle">Kubernetes AI Control Plane</p>
    </div>

    <nav class="sidebar-nav">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="['nav-item', { active: activeTab === tab.id }]"
        @click="emit('tab-change', tab.id)"
      >
        <span class="nav-eyebrow">{{ tab.eyebrow }}</span>
        <span class="nav-label">{{ tab.label }}</span>
      </button>
    </nav>

    <div class="sidebar-footer">
      <div class="cluster-info">
        <p class="meta-label">Active cluster</p>
        <h3>{{ clusterLabel || currentContext }}</h3>
        <button v-if="clusterLabel" class="ghost-btn small" @click="emit('edit-cluster-label')">
          Edit label
        </button>
        <button v-else class="ghost-btn small" @click="emit('edit-cluster-label')">
          Add label
        </button>
      </div>

      <div class="mcp-status">
        <p class="meta-label">MCP endpoint</p>
        <code class="endpoint">{{ endpoint }}</code>
        <div class="status-indicator">
          <span :class="['status-dot', connectivityStatus]"></span>
          <span class="status-text">{{ connectivityStatus === 'green' ? 'Connected' : 'Degraded' }}</span>
        </div>
      </div>

      <button class="sidebar-settings-btn" @click="emit('open-settings')">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M12 15C13.6569 15 15 13.6569 15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15Z" stroke="currentColor" stroke-width="1.5"/>
          <path d="M19.4 15C19.2669 15.3052 19.1339 15.6104 19.0008 15.9155C18.914 16.1171 18.8272 16.3187 18.7404 16.5203C18.6073 16.8255 18.4743 17.1306 18.3412 17.4358C17.8074 18.6241 17.2736 19.8124 16.7398 21.0008C16.499 21.552 16.2582 22.1032 16.0174 22.6544C15.883 22.9577 15.7486 23.261 15.6142 23.5643C15.5274 23.7659 15.4406 23.9675 15.3538 24.1691C15.2207 24.4743 15.0877 24.7794 14.9546 25.0846C14.8215 25.3898 14.6885 25.6949 14.5554 26.0001H9.44458C9.31146 25.6949 9.17835 25.3898 9.04523 25.0846C8.91212 24.7794 8.779 24.4743 8.64589 24.1691C8.55909 23.9675 8.47229 23.7659 8.38549 23.5643C8.25109 23.261 8.11669 22.9577 7.98229 22.6544C7.74149 22.1032 7.50069 21.552 7.25989 21.0008C6.72609 19.8124 6.19229 18.6241 5.65849 17.4358C5.52538 17.1306 5.39226 16.8255 5.25915 16.5203C5.17235 16.3187 5.08555 16.1171 4.99875 15.9155C4.86564 15.6104 4.73252 15.3052 4.59941 15C4.73252 14.6948 4.86564 14.3896 4.99875 14.0845C5.08555 13.8829 5.17235 13.6813 5.25915 13.4797C5.39226 13.1745 5.52538 12.8694 5.65849 12.5642C6.19229 11.3759 6.72609 10.1876 7.25989 9C7.50069 8.448 7.74149 7.8968 7.98229 7.3456C8.11669 7.0423 8.25109 6.739 8.38549 6.4357C8.47229 6.2341 8.55909 6.0325 8.64589 5.8309C8.779 5.5257 8.91212 5.2206 9.04523 4.9154C9.17835 4.6102 9.31146 4.3051 9.44458 4H14.5554C14.6885 4.3051 14.8215 4.6102 14.9546 4.9154C15.0877 5.2206 15.2207 5.5257 15.3538 5.8309C15.4406 6.0325 15.5274 6.2341 15.6142 6.4357C15.7486 6.739 15.883 7.0423 16.0174 7.3456C16.2582 7.8968 16.499 8.448 16.7398 9C17.2736 10.1876 17.8074 11.3759 18.3412 12.5642C18.4743 12.8694 18.6073 13.1745 18.7404 13.4797C18.8272 13.6813 18.914 13.8829 19.0008 14.0845C19.1339 14.3896 19.2669 14.6948 19.4 15Z" stroke="currentColor" stroke-width="1.5"/>
        </svg>
        Settings
      </button>
    </div>

    <button class="collapse-toggle" @click="emit('toggle-collapse')" :title="collapsed ? 'Expand sidebar' : 'Collapse sidebar'">
      {{ collapsed ? '>>' : '<<' }}
    </button>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Tab {
  id: string
  label: string
  eyebrow: string
}

interface Props {
  activeTab: string
  tabs: Tab[]
  currentContext: string
  clusterLabel?: string
  endpoint: string
  connectivityStatus: string
  collapsed?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'tab-change': [id: string]
  'edit-cluster-label': []
  'open-settings': []
  'toggle-collapse': []
}>()
</script>

<style scoped>
.sidebar {
  width: 280px;
  background: var(--sidebar-bg);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 24px;
  gap: 32px;
  height: 100%;
  overflow-y: auto;
  transition: width 0.3s ease;
}

.sidebar.collapsed {
  width: 50px;
  padding: 24px 8px;
}

.sidebar.collapsed .sidebar-header,
.sidebar.collapsed .sidebar-nav,
.sidebar.collapsed .sidebar-footer {
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease;
}

.sidebar-header h2 {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 4px 0;
}

.sidebar-subtitle {
  color: var(--text-secondary);
  font-size: 14px;
  margin: 0;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
  overflow-y: auto;
}

.nav-item {
  text-align: left;
  padding: 12px 16px;
  border-radius: 12px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  transition: all 0.2s;
  cursor: pointer;
}

.nav-item:hover {
  background: var(--hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--active);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.nav-eyebrow {
  display: block;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  opacity: 0.7;
  margin-bottom: 2px;
}

.nav-label {
  display: block;
  font-size: 16px;
  font-weight: 500;
}

.sidebar-footer {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
  flex-shrink: 0;
}

.cluster-info h3 {
  font-size: 18px;
  margin: 4px 0 8px 0;
  font-weight: 500;
}

.mcp-status .endpoint {
  display: block;
  font-family: 'SF Mono', monospace;
  font-size: 12px;
  background: var(--code-bg);
  padding: 6px 8px;
  border-radius: 6px;
  margin: 4px 0 8px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot.green {
  background: var(--success);
}

.status-dot.yellow {
  background: var(--warning);
}

.status-dot.red {
  background: var(--error);
}

.status-text {
  font-size: 14px;
  color: var(--text-secondary);
}

.sidebar-settings-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.sidebar-settings-btn:hover {
  background: var(--hover);
  color: var(--text-primary);
  border-color: var(--border-active);
}

.ghost-btn.small {
  padding: 4px 8px;
  font-size: 12px;
}

.collapse-toggle {
  position: absolute;
  bottom: 20px;
  right: 12px;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  z-index: 20;
}

.collapse-toggle:hover {
  background: var(--hover);
  color: var(--text-primary);
  border-color: var(--border-active);
}

.sidebar.collapsed .collapse-toggle {
  right: 8px;
}

@media (max-width: 880px) {
  .sidebar {
    position: static;
    height: auto;
    width: auto;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
  .sidebar-nav {
    flex: none;
    overflow-y: visible;
  }
}
</style>
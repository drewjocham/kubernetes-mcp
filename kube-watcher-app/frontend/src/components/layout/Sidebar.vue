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

    <!-- Filter Section (shown for anomalies tab) -->
    <div v-if="activeTab === 'anomalies'" class="sidebar-filters">
      <div class="filter-section">
        <p class="meta-label">Severity</p>
        <div class="filter-options">
          <button
            v-for="option in severityOptions"
            :key="option.value"
            :class="['filter-option', { active: selectedSeverity === option.value }]"
            @click="emit('severity-change', option.value)"
            :title="option.label"
          >
            <span class="filter-icon">{{ option.icon }}</span>
            <span class="filter-label">{{ option.label }}</span>
          </button>
        </div>
      </div>

      <div class="filter-section">
        <p class="meta-label">State</p>
        <div class="filter-options">
          <button
            v-for="option in stateOptions"
            :key="option.value"
            :class="['filter-option', { active: selectedState === option.value }]"
            @click="emit('state-change', option.value)"
            :title="option.label"
          >
            <span class="filter-icon">{{ option.icon }}</span>
            <span class="filter-label">{{ option.label }}</span>
          </button>
        </div>
      </div>

      <div v-if="timeRangeOptions" class="filter-section">
        <p class="meta-label">Time Range</p>
        <div class="filter-options">
          <button
            v-for="option in timeRangeOptions"
            :key="option.value"
            :class="['filter-option', { active: selectedTimeRange === option.value }]"
            @click="emit('time-range-change', option.value)"
            :title="option.label"
          >
            <span class="filter-icon">{{ option.icon }}</span>
            <span class="filter-label">{{ option.label }}</span>
          </button>
        </div>
      </div>
    </div>



    <!-- Footer with cluster info and settings -->
    <div class="sidebar-footer">
      <div class="cluster-info">
        <div class="cluster-header">
          <p class="meta-label">Active cluster</p>
          <div class="cluster-title-row">
            <h3>{{ clusterLabel || currentContext }}</h3>
            <div class="status-indicator inline">
              <span :class="['status-dot', connectivityStatus]"></span>
              <span class="status-text">{{ connectivityStatus === 'green' ? 'Connected' : 'Degraded' }}</span>
            </div>
          </div>
        </div>
        <button class="ghost-btn subtle small" @click="emit('edit-cluster-label')">
          {{ clusterLabel ? 'Edit label' : 'Add label' }}
        </button>
      </div>

      <div class="endpoint-section">
        <p class="meta-label">MCP endpoint</p>
        <div class="endpoint-row">
          <code class="endpoint">{{ endpoint }}</code>
          <button class="copy-btn" @click="copyEndpoint" title="Copy to clipboard">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="8" y="8" width="12" height="12" rx="2" stroke="currentColor" stroke-width="1.5"/>
              <rect x="4" y="4" width="12" height="12" rx="2" stroke="currentColor" stroke-width="1.5"/>
            </svg>
          </button>
        </div>
      </div>

      <div class="sidebar-actions">
        <button class="theme-toggle ghost-btn subtle" @click="toggleTheme && toggleTheme()" :title="currentTheme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'">
          <span class="theme-icon">{{ currentTheme === 'light' ? '🌙' : '☀️' }}</span>
          <span class="theme-label">{{ currentTheme === 'light' ? 'Dark' : 'Light' }} mode</span>
        </button>
        <button class="settings-btn" @click="emit('open-settings')" title="Settings">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 15C13.6569 15 15 13.6569 15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15Z" stroke="currentColor" stroke-width="1.5"/>
            <path d="M19.4 15C19.2669 15.3052 19.1339 15.6104 19.0008 15.9155C18.914 16.1171 18.8272 16.3187 18.7404 16.5203C18.6073 16.8255 18.4743 17.1306 18.3412 17.4358C17.8074 18.6241 17.2736 19.8124 16.7398 21.0008C16.499 21.552 16.2582 22.1032 16.0174 22.6544C15.883 22.9577 15.7486 23.261 15.6142 23.5643C15.5274 23.9675 15.4406 23.9675 15.3538 24.1691C15.2207 24.4743 15.0877 24.7794 14.9546 25.0846C14.8215 25.3898 14.6885 25.6949 14.5554 26.0001H9.44458C9.31146 25.6949 9.17835 25.3898 9.04523 25.0846C8.91212 24.7794 8.779 24.4743 8.64589 24.1691C8.55909 23.9675 8.47229 23.7659 8.38549 23.5643C8.25109 23.261 8.11669 22.9577 7.98229 22.6544C7.74149 22.1032 7.50069 21.552 7.25989 21.0008C6.72609 19.8124 6.19229 18.6241 5.65849 17.4358C5.52538 17.1306 5.39226 16.8255 5.25915 16.5203C5.17235 16.3187 5.08555 16.1171 4.99875 15.9155C4.86564 15.6104 4.73252 15.3052 4.59941 15C4.73252 14.6948 4.86564 14.3896 4.99875 14.0845C5.08555 13.8829 5.17235 13.6813 5.25915 13.4797C5.39226 13.1745 5.52538 12.8694 5.65849 12.5642C6.19229 11.3759 6.72609 10.1876 7.25989 9C7.50069 8.448 7.74149 7.8968 7.98229 7.3456C8.11669 7.0423 8.25109 6.739 8.38549 6.4357C8.47229 6.2341 8.55909 6.0325 8.64589 5.8309C8.779 5.5257 8.91212 5.2206 9.04523 4.9154C9.17835 4.6102 9.31146 4.3051 9.44458 4H14.5554C14.6885 4.3051 14.8215 4.6102 14.9546 4.9154C15.0877 5.2206 15.2207 5.5257 15.3538 5.8309C15.4406 6.0325 15.5274 6.2341 15.6142 6.4357C15.7486 6.739 15.883 7.0423 16.0174 7.3456C16.2582 7.8968 16.499 8.448 16.7398 9C17.2736 10.1876 17.8074 11.3759 18.3412 12.5642C18.4743 12.8694 18.6073 13.1745 18.7404 13.4797C18.8272 13.6813 18.914 13.8829 19.0008 14.0845C19.1339 14.3896 19.2669 14.6948 19.4 15Z" stroke="currentColor" stroke-width="1.5"/>
          </svg>
        </button>
      </div>
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

interface Tab {
  id: string
  label: string
  eyebrow: string
}

interface FilterOption {
  value: string
  label: string
  icon: string
}

interface Props {
  activeTab: string
  tabs: Tab[]
  collapsed?: boolean
  severityOptions?: FilterOption[]
  stateOptions?: FilterOption[]
  timeRangeOptions?: FilterOption[]
  selectedSeverity?: string
  selectedState?: string
  selectedTimeRange?: string
  // New props for unified sidebar
  currentContext?: string
  clusterLabel?: string
  endpoint?: string
  connectivityStatus?: string
  toggleTheme?: () => void
  currentTheme?: string
}

const props = withDefaults(defineProps<Props>(), {
  severityOptions: () => [
    { value: 'all', label: 'All', icon: '🌐' },
    { value: 'critical', label: 'Critical', icon: '🔥' },
    { value: 'error', label: 'Error', icon: '❌' },
    { value: 'warning', label: 'Warning', icon: '⚠️' },
    { value: 'info', label: 'Info', icon: 'ℹ️' },
    { value: 'low', label: 'Low', icon: '📉' }
  ],
  stateOptions: () => [
    { value: 'all', label: 'All States', icon: '🌐' },
    { value: 'new', label: 'New', icon: '🆕' },
    { value: 'acknowledged', label: 'Acknowledged', icon: '✅' },
    { value: 'silenced', label: 'Silenced', icon: '🔇' },
    { value: 'being_investigated', label: 'Being Investigated', icon: '🔍' },
    { value: 'false_positive', label: 'False Positive', icon: '👻' },
    { value: 'deleted', label: 'Deleted', icon: '🗑️' }
  ],
  timeRangeOptions: () => [
    { value: '1h', label: 'Last Hour', icon: '⏱️' },
    { value: '24h', label: 'Last 24h', icon: '📅' },
    { value: '7d', label: 'Last 7 Days', icon: '🗓️' },
    { value: '30d', label: 'Last 30 Days', icon: '📆' },
    { value: 'all', label: 'All Time', icon: '∞' }
  ],
  selectedSeverity: 'all',
  selectedState: 'all',
  selectedTimeRange: '24h',
  currentContext: '',
  clusterLabel: undefined,
  endpoint: '',
  connectivityStatus: 'red',
  toggleTheme: undefined,
  currentTheme: 'dark'
})

const emit = defineEmits<{
  'tab-change': [id: string]
  'toggle-collapse': []
  'severity-change': [value: string]
  'state-change': [value: string]
  'time-range-change': [value: string]
  'edit-cluster-label': []
  'open-settings': []
}>()

const copyEndpoint = async () => {
  try {
    await navigator.clipboard.writeText(props.endpoint)
  } catch (err) {
    console.error('Failed to copy endpoint:', err)
  }
}
</script>

<style scoped>
.sidebar {
  position: fixed;
  top: 20px;
  left: 20px;
  bottom: 20px;
  width: 260px;
  background: var(--glass-sidebar);
  backdrop-filter: blur(40px);
  -webkit-backdrop-filter: blur(40px);
  display: flex;
  flex-direction: column;
  justify-content: space-between; /* This keeps Nav at top and Settings at bottom */
  padding: 24px;
  gap: 32px;
  overflow-y: auto;
  transition: width 0.3s ease, left 0.3s ease;
  border-radius: 30px; /* VisionOS curvature */
  border: var(--glass-border-rim);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.05);
  z-index: 100;
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
  border-radius: var(--radius-md);
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

/* Filter Section */
.sidebar-filters {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.filter-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.meta-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin: 0;
}

.filter-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.filter-option {
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.filter-option:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.filter-option.active {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.filter-option.active .filter-icon {
  opacity: 1;
  filter: grayscale(0);
}

.filter-icon {
  font-size: 12px;
  opacity: 0.7;
  filter: grayscale(100%);
  transition: opacity 0.2s, filter 0.2s;
}

.filter-label {
  font-weight: 500;
}

.sidebar-footer {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
  flex-shrink: 0;
  padding-top: 24px;
  border-top: 1px solid var(--border);
}

.sidebar-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
}

.cluster-info h3 {
  font-size: 18px;
  margin: 0;
  font-weight: 500;
}

.endpoint-section .endpoint {
  display: block;
  font-family: 'SF Mono', monospace;
  font-size: 12px;
  background: var(--code-bg);
  padding: 6px 8px;
  border-radius: var(--radius-md);
  margin: 4px 0 8px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
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

.cluster-header {
  margin-bottom: 12px;
}

.cluster-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.status-indicator.inline {
  margin: 0;
}

.endpoint-section {
  margin-top: 16px;
}

.endpoint-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.copy-btn {
  padding: 4px 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.copy-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.ghost-btn.subtle {
  border-color: transparent;
  background: transparent;
  color: var(--text-secondary);
  opacity: 0.8;
}

.ghost-btn.subtle:hover {
  opacity: 1;
  background: var(--hover);
}

.sidebar-settings-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-radius: var(--radius-md);
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
  border-radius: var(--radius-md);
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

.theme-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.theme-toggle:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.theme-icon {
  font-size: 18px;
}

.theme-label {
  font-size: 14px;
  font-weight: 500;
}

.settings-btn {
  padding: 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
}

.settings-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

</style>
<template>
  <aside :class="['sidebar', { collapsed }]">
    <div class="sidebar-header">
      <h2>Argus Console</h2>
      <p class="sidebar-subtitle">Kubernetes AI Control Plane</p>
    </div>
    <hr class="sidebar-divider" />

    <nav class="sidebar-nav">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="['nav-item', { active: activeTab === tab.id }]"
        @click="emit('tab-change', tab.id)"
      >
        <span class="nav-icon">
          <component :is="tab.icon" size="18" weight="fill" v-if="tab.icon" />
        </span>
        <span class="nav-eyebrow" v-if="tab.eyebrow">{{ tab.eyebrow }}</span>
        <span class="nav-label">{{ tab.label }}</span>
      </button>
    </nav>

    <!-- Filter Section (always visible) -->
    <div class="sidebar-filters">
      <div class="filter-section">
        <p class="meta-label">Severity</p>
        <div class="filter-options severity-options">
          <button
            v-for="option in severityOptions"
            :key="option.value"
            :class="['filter-option', { active: selectedSeverity === option.value }]"
            :data-value="option.value"
            @click="emit('severity-change', option.value)"
            :title="option.label"
          >
            <span class="filter-icon">
              <component :is="option.icon" size="12" weight="fill" v-if="option.icon" />
            </span>
            <span class="filter-label">{{ option.label }}</span>
          </button>
        </div>
      </div>

      <div class="filter-section">
        <p class="meta-label">State</p>
        <select 
          :value="selectedState" 
          @change="emit('state-change', ($event.target as HTMLSelectElement).value)"
          class="select-input"
        >
          <option v-for="option in stateOptions" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </div>

      <div v-if="timeRangeOptions" class="filter-section">
        <p class="meta-label">Time Range</p>
        <div class="filter-options time-range-options">
          <button
            v-for="option in timeRangeOptions"
            :key="option.value"
            :class="['filter-option', { active: selectedTimeRange === option.value }]"
            @click="emit('time-range-change', option.value)"
            :title="option.label"
          >
            <span class="filter-icon">
              <component :is="option.icon" size="14" weight="fill" v-if="option.icon" />
            </span>
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
            <PhCopy size="16" weight="regular" />
          </button>
        </div>
      </div>

      <div class="sidebar-actions">
         <button class="theme-toggle ghost-btn subtle" @click="toggleTheme && toggleTheme()" :title="currentTheme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'">
           <span class="theme-icon">
             <PhMoon size="18" weight="regular" v-if="currentTheme === 'light'" />
             <PhSun size="18" weight="regular" v-else />
           </span>
           <span class="theme-label">{{ currentTheme === 'light' ? 'Dark' : 'Light' }} mode</span>
         </button>
         <button class="settings-btn" @click="emit('open-settings')" title="Settings">
           <PhGear size="20" weight="regular" />
         </button>
      </div>
    </div>

    <button class="collapse-toggle" @click="emit('toggle-collapse')" :title="collapsed ? 'Expand sidebar' : 'Collapse sidebar'">
      <PhCaretRight size="16" weight="regular" v-if="collapsed" />
      <PhCaretLeft size="16" weight="regular" v-else />
    </button>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  PhRobot,
  PhWarning,
  PhLightbulb,
  PhScroll,
  PhEye,
  PhGlobe,
  PhFire,
  PhXCircle,
  PhWarningCircle,
  PhInfo,
  PhChartLineDown,
  PhCalendarBlank,
  PhCalendar,
  PhCalendarDots,
  PhInfinity,
  PhCopy,
  PhMoon,
  PhSun,
  PhGear,
  PhCaretLeft,
  PhCaretRight,
  PhSparkle,
  PhCheckCircle,
  PhSpeakerSlash,
  PhMagnifyingGlass,
  PhGhost,
  PhTrash
} from '@phosphor-icons/vue'

interface Tab {
  id: string
  label: string
  eyebrow: string
  icon?: any
}

interface FilterOption {
  value: string
  label: string
  icon: any
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
    { value: 'all', label: 'All', icon: PhGlobe },
    { value: 'critical', label: 'Critical', icon: PhFire },
    { value: 'error', label: 'Error', icon: PhXCircle },
    { value: 'warning', label: 'Warning', icon: PhWarningCircle },
    { value: 'info', label: 'Info', icon: PhInfo },
    { value: 'low', label: 'Low', icon: PhChartLineDown }
  ],
  stateOptions: () => [
    { value: 'all', label: 'All States', icon: PhGlobe },
    { value: 'new', label: 'New', icon: PhSparkle },
    { value: 'acknowledged', label: 'Acknowledged', icon: PhCheckCircle },
    { value: 'silenced', label: 'Silenced', icon: PhSpeakerSlash },
    { value: 'being_investigated', label: 'Being Investigated', icon: PhMagnifyingGlass },
    { value: 'false_positive', label: 'False Positive', icon: PhGhost },
    { value: 'deleted', label: 'Deleted', icon: PhTrash }
  ],
  timeRangeOptions: () => [
    { value: '1h', label: '1h', icon: PhCalendarBlank },
    { value: '24h', label: '24h', icon: PhCalendar },
    { value: '7d', label: '7d', icon: PhCalendarDots },
    { value: '30d', label: '30d', icon: PhCalendarDots },
    { value: 'all', label: 'All', icon: PhInfinity }
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
  backdrop-filter: blur(60px);
  -webkit-backdrop-filter: blur(60px);
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: width 0.3s ease;
}

.sidebar-header {
  margin-bottom: 4px;
}

.sidebar-subtitle {
  font-size: 12px;
  color: var(--text-tertiary);
  margin: 2px 0 0 0;
  font-weight: 400;
}

.sidebar.collapsed {
  width: 70px;
}

.sidebar.collapsed .sidebar-header,
.sidebar.collapsed .sidebar-filters,
.sidebar.collapsed .sidebar-footer {
  display: none;
}

.sidebar.collapsed .sidebar-nav {
  gap: 4px;
}

.sidebar.collapsed .nav-item {
  padding: 12px;
  justify-content: center;
}

.sidebar.collapsed .nav-label,
.sidebar.collapsed .nav-eyebrow {
  display: none;
}

.sidebar.collapsed .nav-icon {
  width: 24px;
  height: 24px;
  font-size: 20px;
}

.filter-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.meta-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-tertiary);
  margin: 0 0 6px 0;
  font-weight: 600;
}

.filter-options {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 4px;
  border-radius: var(--radius-lg);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.severity-options {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  grid-template-rows: repeat(2, auto);
  gap: 4px;
  padding: 4px;
  border-radius: var(--radius-lg);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.severity-options .filter-option {
  padding: 8px 4px;
  border-radius: var(--radius-md);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  position: relative;
}

.severity-options .filter-option::before {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--severity-color, var(--text-tertiary));
  display: block;
}

.severity-options .filter-option[data-value="critical"]::before { background: var(--danger); }
.severity-options .filter-option[data-value="error"]::before { background: var(--warning); }
.severity-options .filter-option[data-value="warning"]::before { background: var(--warning); }
.severity-options .filter-option[data-value="info"]::before { background: var(--info); }
.severity-options .filter-option[data-value="low"]::before { background: var(--text-tertiary); }
.severity-options .filter-option[data-value="all"]::before { background: var(--primary); }

.severity-options .filter-label {
  font-size: 10px;
  font-weight: 500;
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  max-width: 100%;
}

.severity-options .filter-option.active {
  background: rgba(255, 255, 255, 0.15);
  color: var(--text-primary);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.severity-options .filter-option.active::before {
  transform: scale(1.2);
}

.severity-options .filter-icon {
  display: none;
}

.filter-option {
  padding: 6px 10px;
  border-radius: var(--radius-md);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
  flex: 1;
  justify-content: center;
}

.filter-option:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.filter-option.active {
  background: rgba(255, 255, 255, 0.15);
  color: var(--text-primary);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
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

.time-range-options {
  display: flex;
  flex-wrap: nowrap;
  gap: 2px;
  padding: 4px;
  border-radius: var(--radius-lg);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.time-range-options .filter-option {
  flex: 1;
  padding: 6px 2px;
  border-radius: var(--radius-md);
  font-size: 11px;
  text-align: center;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.time-range-options .filter-option.active {
  background: var(--primary);
  color: white;
}

.time-range-options .filter-icon {
  display: none;
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
  font-size: 20px;
  margin: 0 0 4px 0;
  font-weight: 600;
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
  box-shadow: 0 0 8px var(--success);
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

.sidebar-divider {
  border: none;
  height: 1px;
  background: var(--border);
  margin: 8px 0;
  opacity: 0.5;
}

.select-input {
  width: 100%;
  padding: 12px 16px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  color: var(--text);
  font-size: 14px;
  font-family: inherit;
  appearance: none;
  cursor: pointer;
  transition: all 0.2s;
}

.select-input:hover {
  background: rgba(255, 255, 255, 0.15);
  border-color: var(--border-active);
}

.select-input:focus {
  outline: none;
  border-color: var(--border-active);
  box-shadow: 0 0 0 2px rgba(var(--primary-rgb), 0.2);
}

/* Navigation styles */
.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: left;
}

.nav-item:hover {
  background: var(--hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--primary);
  color: white;
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  font-size: 18px;
  opacity: 0.8;
  transition: opacity 0.2s;
}

.nav-item.active .nav-icon {
  opacity: 1;
  color: white;
}

.nav-eyebrow {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-tertiary);
  margin-bottom: 2px;
}

.nav-label {
  font-weight: 500;
  flex: 1;
}

</style>
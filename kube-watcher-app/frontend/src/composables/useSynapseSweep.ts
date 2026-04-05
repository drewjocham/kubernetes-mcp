import { ref, watch, nextTick, computed, onUnmounted } from 'vue'
import { RunSynapseSweep } from '../../wailsjs/go/main/App'

type ScanHealth = 'good' | 'warning' | 'unhealthy'

type ScanCell = {
  title: string
  summary: string
  health: ScanHealth
  detail: string
  service: string
}

export function useSynapseSweep() {
  const synapseSweepOutput = ref('')
  const isRunningSweep = ref(false)
  const sweepInterval = ref('manual')
  const customIntervalValue = ref(5)
  const customIntervalUnit = ref('m')
  const nextSweepTime = ref<number | null>(null)
  const countdownText = ref('')
  let sweepTimer: any = null
  let countdownTimer: any = null

  onUnmounted(() => {
    if (sweepTimer) clearInterval(sweepTimer)
    if (countdownTimer) clearInterval(countdownTimer)
  })

  const updateCountdown = () => {
    if (!nextSweepTime.value) {
      countdownText.value = ''
      return
    }

    const now = Date.now()
    const diff = nextSweepTime.value - now

    if (diff <= 0) {
      countdownText.value = 'Cluster scan is ready'
      return
    }

    const seconds = Math.floor(diff / 1000)
    const minutes = Math.floor(seconds / 60)
    const hours = Math.floor(minutes / 60)
    const days = Math.floor(hours / 24)

    let timeStr = ''
    if (days > 0) {
      timeStr = `${days}d ${hours % 24}h`
    } else if (hours > 0) {
      timeStr = `${hours}h ${minutes % 60}m`
    } else if (minutes > 0) {
      timeStr = `${minutes}m ${seconds % 60}s`
    } else {
      timeStr = `${seconds}s`
    }

    countdownText.value = `Next scan in ${timeStr}`
  }

  watch(sweepInterval, (newVal: string) => {
    if (sweepTimer) {
      clearInterval(sweepTimer)
      sweepTimer = null
    }
    if (countdownTimer) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
    nextSweepTime.value = null
    countdownText.value = ''

    if (newVal === 'manual') return

    let ms = 0
    if (newVal === 'custom') {
      const unitMsMap: Record<string, number> = {
        'm': 60 * 1000,
        'h': 60 * 60 * 1000,
        'd': 24 * 60 * 60 * 1000,
      }
      ms = (customIntervalValue.value || 0) * (unitMsMap[customIntervalUnit.value] || 60000)
    } else {
      const msMap: Record<string, number> = {
        '5m': 5 * 60 * 1000,
        '30m': 30 * 60 * 1000,
        '2h': 2 * 60 * 60 * 1000,
        '5h': 5 * 60 * 60 * 1000,
        '12h': 12 * 60 * 60 * 1000,
        '1d': 24 * 60 * 60 * 1000,
      }
      ms = msMap[newVal]
    }

    if (ms > 0) {
      nextSweepTime.value = Date.now() + ms
      updateCountdown()
      countdownTimer = setInterval(updateCountdown, 1000)

      sweepTimer = setInterval(() => {
        if (!isRunningSweep.value) {
          runSynapseSweep()
          nextSweepTime.value = Date.now() + ms
        }
      }, ms)
    }
  })

  watch([customIntervalValue, customIntervalUnit], () => {
    if (sweepInterval.value === 'custom') {
      // Trigger the sweepInterval watcher
      const val = sweepInterval.value
      sweepInterval.value = 'manual'
      nextTick(() => {
        sweepInterval.value = val
      })
    }
  })

  const synapseSweepGrid = computed(() => buildScanGrid(synapseSweepOutput.value))

  async function runSynapseSweep() {
    isRunningSweep.value = true
    synapseSweepOutput.value = ''
    try {
      const result = await RunSynapseSweep()
      console.log('RunSynapseSweep raw result length:', result.length, 'first 200 chars:', result.slice(0, 200))
      synapseSweepOutput.value = result
    } catch (error) {
      console.error('RunSynapseSweep error:', error)
      synapseSweepOutput.value = formatError(error)
    } finally {
      isRunningSweep.value = false
    }
  }

  function formatError(error: unknown): string {
    return error instanceof Error ? error.message : String(error)
  }

  // Helper functions for scan grid (copied from App.vue)
  function buildScanGrid(raw: string) {
    const trimmed = raw.trim()
    if (!trimmed) {
      return { cells: [] as ScanCell[], headline: '', fallback: '' }
    }

    const parsed = tryParseSynapseSweep(trimmed)
    if (!parsed) {
      return { cells: [] as ScanCell[], headline: '', fallback: raw }
    }

    const source = readRecord(parsed)
    const report = readRecord(source.popeye) || source
    const sections = Array.isArray(report.sections) ? report.sections : []
    if (sections.length === 0) {
      return { cells: [] as ScanCell[], headline: '', fallback: raw }
    }

    const cells = sections
      .map((entry, index) => buildScanCell(entry, index))
      .filter((entry): entry is ScanCell => entry !== null)
      .sort((left, right) => healthRank(left.health) - healthRank(right.health))

    if (cells.length === 0) {
      return { cells: [] as ScanCell[], headline: '', fallback: raw }
    }

    const score = readRecord(report.score)
    const grade = readString(score.grade)
    const totalIssues = cells.filter((cell) => cell.health !== 'good').length
    const headline = grade
      ? `Grade ${grade} · ${totalIssues} areas need attention`
      : `${totalIssues} areas need attention`

    return { cells, headline, fallback: '' }
  }

  function tryParseSynapseSweep(raw: string) {
    try {
      return JSON.parse(raw)
    } catch {
      const start = raw.indexOf('{')
      const end = raw.lastIndexOf('}')
      if (start === -1 || end === -1 || end <= start) return null
      try {
        return JSON.parse(raw.slice(start, end + 1))
      } catch {
        return null
      }
    }
  }

  function buildScanCell(entry: unknown, index: number): ScanCell | null {
    const record = readRecord(entry)
    const tally = readRecord(record.tally)
    const name = readString(record.linter) || readString(record.gvr) || `Check ${index + 1}`
    const errors = readNumber(tally.error)
    const warnings = readNumber(tally.warning)
    const passing = readNumber(tally.ok)
    const infos = readNumber(tally.info)
    const detail = buildScanDetail(name, tally, record.issues)

    if (!detail && errors === 0 && warnings === 0 && passing === 0 && infos === 0) {
      return null
    }

    return {
      title: shortenLabel(name),
      summary:
        errors > 0
          ? `${errors} errors`
          : warnings > 0
            ? `${warnings} warnings`
            : `${passing} healthy`,
      health: errors > 0 ? 'unhealthy' : warnings > 0 ? 'warning' : 'good',
      detail: detail || `${name}\nNo detailed findings were reported.`,
      service: name.includes('/') ? name.split('/')[0] : 'cluster',
    }
  }

  function buildScanDetail(name: string, tally: Record<string, unknown>, issues: unknown) {
    const parts = [name]
    const counters = [
      readNumber(tally.error) > 0 ? `${readNumber(tally.error)} errors` : '',
      readNumber(tally.warning) > 0 ? `${readNumber(tally.warning)} warnings` : '',
      readNumber(tally.ok) > 0 ? `${readNumber(tally.ok)} passing` : '',
      readNumber(tally.info) > 0 ? `${readNumber(tally.info)} info` : '',
    ].filter(Boolean)

    if (counters.length > 0) {
      parts.push(counters.join(' · '))
    }

    const findings = flattenPopeyeIssues(issues).slice(0, 3)
    if (findings.length > 0) {
      parts.push(findings.join('\n'))
    }

    return parts.join('\n')
  }

  function flattenPopeyeIssues(value: unknown): string[] {
    if (Array.isArray(value)) {
      return value.flatMap((entry) => flattenPopeyeIssues(entry))
    }

    const record = readRecord(value)
    const message = readString(record.message)
    if (message) {
      const group = readString(record.group)
      const prefix = group && group !== '__root__' ? `${group}: ` : ''
      return [`${prefix}${message}`]
    }

    // Handle nested issues in popeye format
    if (record.issues && Array.isArray(record.issues)) {
      return record.issues.flatMap((issue: unknown) => flattenPopeyeIssues(issue))
    }

    return []
  }

  function readRecord(value: unknown): Record<string, unknown> {
    return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
  }

  function readString(value: unknown) {
    return typeof value === 'string' ? value.trim() : ''
  }

  function readNumber(value: unknown) {
    if (typeof value === 'number' && Number.isFinite(value)) return value
    if (typeof value === 'string') {
      const parsed = Number(value)
      return Number.isFinite(parsed) ? parsed : 0
    }
    return 0
  }

  function healthRank(value: ScanHealth) {
    if (value === 'unhealthy') return 0
    if (value === 'warning') return 1
    return 2
  }

  function shortenLabel(value: string) {
    return value.length > 28 ? `${value.slice(0, 25)}...` : value
  }

  return {
    synapseSweepOutput,
    isRunningSweep,
    sweepInterval,
    customIntervalValue,
    customIntervalUnit,
    nextSweepTime,
    countdownText,
    synapseSweepGrid,
    runSynapseSweep,
  }
}
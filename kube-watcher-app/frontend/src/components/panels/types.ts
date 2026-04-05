export interface ScanCell {
  title: string
  summary: string
  health: 'good' | 'warning' | 'unhealthy'
  detail: string
  service: string
}
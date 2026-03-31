import { reactive } from 'vue'

export const logStore = reactive<Record<string, string>>({})

export function setLogs(command: string, logs: string) {
  logStore[command] = logs
}

export function getLogs(command: string): string | undefined {
  return logStore[command]
}

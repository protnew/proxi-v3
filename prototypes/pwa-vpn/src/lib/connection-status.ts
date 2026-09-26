/** CON-001: Connection status UX model */
export type ConnPhase =
  | 'idle'
  | 'searching_path'
  | 'connecting'
  | 'connected'
  | 'peer_offline'
  | 'retrying'
  | 'failed'

export interface ConnStatus {
  phase: ConnPhase
  labelRu: string
  labelEn: string
  retryInSec?: number
  detail?: string
}

const LABELS: Record<ConnPhase, { ru: string; en: string }> = {
  idle: { ru: 'Не подключено', en: 'Disconnected' },
  searching_path: { ru: 'Ищем путь…', en: 'Searching path…' },
  connecting: { ru: 'Соединяемся…', en: 'Connecting…' },
  connected: { ru: 'Связь есть', en: 'Connected' },
  peer_offline: { ru: 'Peer оффлайн', en: 'Peer offline' },
  retrying: { ru: 'Повтор…', en: 'Retrying…' },
  failed: { ru: 'Не удалось', en: 'Failed' },
}

export function makeStatus(phase: ConnPhase, opts?: { retryInSec?: number; detail?: string }): ConnStatus {
  const L = LABELS[phase]
  return {
    phase,
    labelRu: L.ru,
    labelEn: L.en,
    retryInSec: opts?.retryInSec,
    detail: opts?.detail,
  }
}

/** Simple auto-retry countdown helper */
export function nextRetrySeconds(attempt: number, base = 2, cap = 30): number {
  return Math.min(cap, base * Math.pow(2, Math.max(0, attempt)))
}

export function phaseFromVpnUi(status: string): ConnPhase {
  switch (status) {
    case 'connected':
    case 'sharing':
      return 'connected'
    case 'connecting':
      return 'connecting'
    case 'error':
    case 'core_down':
      return 'failed'
    case 'locked':
      return 'peer_offline'
    default:
      return 'idle'
  }
}

export function phaseFromDesktop(status: string): ConnPhase {
  if (status === 'unavailable') return 'idle'
  return phaseFromVpnUi(status)
}

import { useEffect, useRef, useState } from 'react'
import type { DashboardData } from '@/types/api'

export type WsStatus = 'connecting' | 'connected' | 'reconnecting'

export function useDashboard() {
  const [data, setData] = useState<DashboardData | null>(null)
  const [status, setStatus] = useState<WsStatus>('connecting')
  const retryDelay = useRef(1000)

  useEffect(() => {
    let ws: WebSocket
    let timeoutId: ReturnType<typeof setTimeout>
    let cancelled = false

    function connect() {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
      ws = new WebSocket(`${proto}//${location.host}/api/v1/ws`)

      ws.onopen = () => {
        if (cancelled) { ws.close(); return }
        setStatus('connected')
        retryDelay.current = 1000
      }

      ws.onmessage = (e) => {
        if (cancelled) return
        try { setData(JSON.parse(e.data as string) as DashboardData) } catch { /* ignore */ }
      }

      ws.onclose = () => {
        if (cancelled) return
        setStatus('reconnecting')
        timeoutId = setTimeout(() => {
          retryDelay.current = Math.min(retryDelay.current * 2, 30000)
          connect()
        }, retryDelay.current)
      }
    }

    connect()

    return () => {
      cancelled = true
      clearTimeout(timeoutId)
      ws?.close()
    }
  }, [])

  return { data, status }
}

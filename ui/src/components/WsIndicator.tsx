import type { WsStatus } from '@/lib/use-dashboard'

export function WsIndicator({ status }: { status: WsStatus }) {
  if (status === 'connected') return null
  return (
    <div className="fixed bottom-4 right-4 z-50 rounded-full bg-yellow-500/90 px-3 py-1 text-xs font-medium text-white shadow-lg">
      {status === 'connecting' ? 'Connecting…' : 'Reconnecting…'}
    </div>
  )
}

import { useMemo, useState, useEffect } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { format, isToday, isTomorrow, parseISO } from 'date-fns'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { fetchAlerts, fetchAlertPreferences, putAlertPreferences, apiMuteAlert, apiUnmuteAlert } from '@/lib/api'

const TYPE_LABELS: Record<string, string> = {
  threshold_breach: 'Threshold',
  short_cycle: 'Short Cycle',
  scheduled_check: 'Scheduled',
  differential: 'Differential',
  count_per_window: 'Count',
  cycle_complete: 'Cycle',
}

function formatMutedUntil(iso: string): string {
  const d = parseISO(iso)
  if (isToday(d)) return `until ${format(d, 'h:mm a')}`
  if (isTomorrow(d)) return `until tomorrow at ${format(d, 'h:mm a')}`
  return `until ${format(d, 'MMM d, h:mm a')}`
}

export function AlertsPage() {
  const qc = useQueryClient()
  const { data: alerts = [] } = useQuery({ queryKey: ['alerts'], queryFn: fetchAlerts })
  const { data: prefs } = useQuery({ queryKey: ['alert-preferences'], queryFn: fetchAlertPreferences })

  const [pending, setPending] = useState<Set<string>>(new Set())
  const [saving, setSaving] = useState(false)
  const [feedback, setFeedback] = useState<'saved' | 'error' | null>(null)
  const [mutingKey, setMutingKey] = useState<string | null>(null)

  useEffect(() => {
    if (prefs) setPending(new Set(prefs.subscribed))
  }, [prefs])

  const isDirty = useMemo(() => {
    if (!prefs) return false
    const orig = new Set(prefs.subscribed)
    if (orig.size !== pending.size) return true
    for (const k of pending) if (!orig.has(k)) return true
    return false
  }, [pending, prefs])

  function togglePending(key: string) {
    setMutingKey(null)
    setPending(prev => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  async function handleSave() {
    setSaving(true)
    setFeedback(null)
    try {
      await putAlertPreferences(Array.from(pending))
      await qc.invalidateQueries({ queryKey: ['alert-preferences'] })
      setFeedback('saved')
    } catch {
      setFeedback('error')
    } finally {
      setSaving(false)
      setTimeout(() => setFeedback(null), 3000)
    }
  }

  async function handleMute(key: string, duration: string) {
    setMutingKey(null)
    try {
      await apiMuteAlert(key, duration)
      await qc.invalidateQueries({ queryKey: ['alert-preferences'] })
    } catch { /* ignore — server will be retried on next load */ }
  }

  async function handleUnmute(key: string) {
    try {
      await apiUnmuteAlert(key)
      await qc.invalidateQueries({ queryKey: ['alert-preferences'] })
    } catch { /* ignore */ }
  }

  const mutes = prefs?.mutes ?? {}

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Alerts</h1>
        <div className="flex items-center gap-3">
          {feedback === 'saved' && (
            <span className="text-sm text-green-600 dark:text-green-400">Saved</span>
          )}
          {feedback === 'error' && (
            <span className="text-sm text-destructive">Save failed</span>
          )}
          <Button onClick={() => void handleSave()} disabled={!isDirty || saving} size="sm">
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>

      <Card>
        <CardContent className="divide-y p-0">
          {alerts.length === 0 && (
            <p className="px-4 py-6 text-sm text-muted-foreground">No alerts configured.</p>
          )}
          {alerts.map(alert => {
            const subscribed = pending.has(alert.key)
            const muteUntil = mutes[alert.key]
            const isMuted = !!muteUntil
            const showingPicker = mutingKey === alert.key

            return (
              <div
                key={alert.key}
                className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    id={alert.key}
                    checked={subscribed}
                    onChange={() => togglePending(alert.key)}
                    className="h-4 w-4 rounded border-border accent-primary"
                  />
                  <div className="flex items-center gap-2">
                    <Label htmlFor={alert.key} className="cursor-pointer font-medium leading-none">
                      {alert.name}
                    </Label>
                    <Badge variant="secondary" className="text-xs font-normal">
                      {TYPE_LABELS[alert.type] ?? alert.type}
                    </Badge>
                  </div>
                </div>

                {subscribed && (
                  <div className="flex items-center gap-2 pl-7 sm:pl-0">
                    {isMuted ? (
                      <>
                        <span className="text-xs text-muted-foreground">
                          Muted {formatMutedUntil(muteUntil)}
                        </span>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => void handleUnmute(alert.key)}
                        >
                          Unmute
                        </Button>
                      </>
                    ) : showingPicker ? (
                      <>
                        <Button variant="outline" size="sm" onClick={() => void handleMute(alert.key, '1h')}>1h</Button>
                        <Button variant="outline" size="sm" onClick={() => void handleMute(alert.key, '4h')}>4h</Button>
                        <Button variant="outline" size="sm" onClick={() => void handleMute(alert.key, 'tomorrow')}>Tomorrow</Button>
                        <Button variant="ghost" size="sm" onClick={() => setMutingKey(null)}>Cancel</Button>
                      </>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setMutingKey(alert.key)}
                      >
                        Mute
                      </Button>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}

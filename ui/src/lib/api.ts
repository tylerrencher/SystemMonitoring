import type { AlertItem, AlertPreferences, ChartPoint, SolarChartPoint, User, WeatherChartPoint } from '@/types/api'

export async function apiLogin(username: string, password: string): Promise<{ user: User }> {
  const res = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) throw new Error('Invalid credentials')
  return res.json() as Promise<{ user: User }>
}

export async function apiLogout(): Promise<void> {
  await fetch('/api/v1/auth/logout', { method: 'POST' })
}

export async function fetchSeries(): Promise<string[]> {
  const res = await fetch('/api/v1/series')
  if (!res.ok) throw new Error('Failed to fetch series')
  return res.json() as Promise<string[]>
}

export async function fetchInverters(): Promise<string[]> {
  const res = await fetch('/api/v1/solar/inverters')
  if (!res.ok) throw new Error('Failed to fetch inverters')
  return res.json() as Promise<string[]>
}

export async function fetchPowerChart(params: {
  series: string
  from?: string
  to?: string
}): Promise<ChartPoint[]> {
  const url = new URL('/api/v1/charts/power', window.location.origin)
  url.searchParams.set('series', params.series)
  if (params.from) url.searchParams.set('from', params.from)
  if (params.to) url.searchParams.set('to', params.to)
  const res = await fetch(url)
  if (!res.ok) throw new Error('Query failed')
  return res.json() as Promise<ChartPoint[]>
}

export async function fetchSolarChart(params: {
  metric: string
  inverter?: string
  from?: string
  to?: string
}): Promise<SolarChartPoint[]> {
  const url = new URL('/api/v1/charts/solar', window.location.origin)
  url.searchParams.set('metric', params.metric)
  if (params.inverter) url.searchParams.set('inverter', params.inverter)
  if (params.from) url.searchParams.set('from', params.from)
  if (params.to) url.searchParams.set('to', params.to)
  const res = await fetch(url)
  if (!res.ok) throw new Error('Query failed')
  return res.json() as Promise<SolarChartPoint[]>
}

export async function fetchAlerts(): Promise<AlertItem[]> {
  const res = await fetch('/api/v1/alerts')
  if (!res.ok) throw new Error('Failed to fetch alerts')
  return res.json() as Promise<AlertItem[]>
}

export async function fetchAlertPreferences(): Promise<AlertPreferences> {
  const res = await fetch('/api/v1/alerts/preferences')
  if (!res.ok) throw new Error('Failed to fetch preferences')
  return res.json() as Promise<AlertPreferences>
}

export async function putAlertPreferences(keys: string[]): Promise<void> {
  const res = await fetch('/api/v1/alerts/preferences', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keys }),
  })
  if (!res.ok) throw new Error('Failed to save preferences')
}

export async function apiMuteAlert(key: string, duration: string): Promise<void> {
  const res = await fetch(`/api/v1/alerts/${encodeURIComponent(key)}/mute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ duration }),
  })
  if (!res.ok) throw new Error('Failed to mute alert')
}

export async function apiUnmuteAlert(key: string): Promise<void> {
  const res = await fetch(`/api/v1/alerts/${encodeURIComponent(key)}/mute`, { method: 'DELETE' })
  if (!res.ok) throw new Error('Failed to unmute alert')
}

export async function fetchWeatherChart(params: {
  metric: string
  from?: string
  to?: string
}): Promise<WeatherChartPoint[]> {
  const url = new URL('/api/v1/charts/weather', window.location.origin)
  url.searchParams.set('metric', params.metric)
  if (params.from) url.searchParams.set('from', params.from)
  if (params.to) url.searchParams.set('to', params.to)
  const res = await fetch(url)
  if (!res.ok) throw new Error('Query failed')
  return res.json() as Promise<WeatherChartPoint[]>
}

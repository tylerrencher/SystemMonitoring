import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatCard } from '@/components/StatCard'
import { WsIndicator } from '@/components/WsIndicator'
import { useDashboard } from '@/lib/use-dashboard'
import type { Consumer } from '@/types/api'

const SOURCE_LABELS: Record<string, string> = {
  solar_only: 'Solar',
  battery_solar: 'Solar + Battery',
  battery_only: 'Battery',
  generator: 'Generator',
}

const SOURCE_VARIANTS: Record<string, 'default' | 'secondary' | 'outline' | 'destructive'> = {
  solar_only: 'default',
  battery_solar: 'secondary',
  battery_only: 'outline',
  generator: 'destructive',
}

function fmt(w: number) {
  return Math.abs(w) >= 1000 ? `${(w / 1000).toFixed(1)}k` : String(Math.round(w))
}

function ConsumerRow({ consumer, maxWatts }: { consumer: Consumer; maxWatts: number }) {
  const pct = maxWatts > 0 ? (consumer.avg_watts / maxWatts) * 100 : 0
  return (
    <div className="flex items-center gap-3">
      <span className="w-28 shrink-0 truncate text-right text-sm">{consumer.series}</span>
      <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
        <div className="h-2 rounded-full bg-primary transition-all" style={{ width: `${pct}%` }} />
      </div>
      <span className="w-16 shrink-0 text-right text-sm text-muted-foreground">
        {fmt(consumer.avg_watts)} W
      </span>
    </div>
  )
}

export function DashboardPage() {
  const { data, status } = useDashboard()

  const maxConsumer = data?.top_consumers[0]?.avg_watts ?? 1

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        {data && (
          <Badge variant={SOURCE_VARIANTS[data.power_source] ?? 'outline'}>
            {SOURCE_LABELS[data.power_source] ?? data.power_source}
          </Badge>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Solar"
          value={fmt(data?.pv_power_w ?? 0)}
          unit="W"
        />
        <StatCard
          label="Battery"
          value={fmt(data?.battery_power_w ?? 0)}
          unit="W"
          sub={data ? `${Math.round(data.battery_soc_pct)}% SoC` : undefined}
        />
        <StatCard
          label="Generator"
          value={fmt(data?.generator_power_w ?? 0)}
          unit="W"
        />
        <StatCard
          label="Consumption"
          value={fmt(data?.total_consumption_w ?? 0)}
          unit="W"
        />
      </div>

      {data?.weather && (
        <div className="grid gap-4 sm:grid-cols-2">
          <StatCard
            label="Temperature"
            value={data.weather.temp_f != null ? data.weather.temp_f.toFixed(1) : '—'}
            unit="°F"
          />
          <StatCard
            label="Humidity"
            value={data.weather.humidity != null ? Math.round(data.weather.humidity) : '—'}
            unit="%"
          />
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Top Consumers</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {data?.top_consumers.length ? (
            data.top_consumers.map((c) => (
              <ConsumerRow key={`${c.device}-${c.series}`} consumer={c} maxWatts={maxConsumer} />
            ))
          ) : (
            <p className="text-sm text-muted-foreground">No data</p>
          )}
        </CardContent>
      </Card>

      <WsIndicator status={status} />
    </div>
  )
}

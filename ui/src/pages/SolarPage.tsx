import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ChartTimeRange, type TimeRange } from '@/components/ChartTimeRange'
import { fetchInverters, fetchSolarChart } from '@/lib/api'
import { makeXAxisFormatter } from '@/lib/utils'
import { subHours } from 'date-fns'

const METRICS = [
  { value: 'pv_power', label: 'PV Power' },
  { value: 'battery_power', label: 'Battery Power' },
  { value: 'load_power', label: 'Load Power' },
  { value: 'battery_state_of_charge', label: 'Battery SoC' },
]

function defaultRange(): TimeRange {
  const to = new Date()
  return { from: subHours(to, 24).toISOString(), to: to.toISOString() }
}

export function SolarPage() {
  const [metric, setMetric] = useState('pv_power')
  const [inverter, setInverter] = useState('all')
  const [range, setRange] = useState<TimeRange>(defaultRange)

  const { data: inverters = [] } = useQuery({
    queryKey: ['inverters'],
    queryFn: fetchInverters,
  })

  const isAll = inverter === 'all'

  const { data: points = [], isFetching } = useQuery({
    queryKey: ['solar-chart', metric, inverter, range],
    queryFn: () =>
      fetchSolarChart({
        metric,
        inverter: isAll ? undefined : inverter,
        from: range.from,
        to: range.to,
      }),
  })

  const metricLabel = METRICS.find((m) => m.value === metric)?.label ?? metric
  const unit = metric === 'battery_state_of_charge' ? '%' : 'W'

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Solar</h1>

      <div className="flex flex-wrap items-center gap-4">
        <Select value={metric} onValueChange={setMetric}>
          <SelectTrigger className="w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {METRICS.map((m) => (
              <SelectItem key={m.value} value={m.value}>
                {m.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={inverter} onValueChange={setInverter}>
          <SelectTrigger className="w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All (combined)</SelectItem>
            {inverters.map((inv) => (
              <SelectItem key={inv} value={inv}>
                {inv}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <ChartTimeRange onChange={setRange} />
      </div>

      <div className="h-80">
        {isFetching ? (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            Loading…
          </div>
        ) : points.length === 0 ? (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            No data for selected range.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={points} margin={{ top: 4, right: 16, bottom: 4, left: 0 }}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis
                dataKey="time"
                tickFormatter={makeXAxisFormatter(range.from, range.to)}
                tick={{ fontSize: 11 }}
                minTickGap={60}
              />
              <YAxis
                tick={{ fontSize: 11 }}
                tickFormatter={(value: number) => `${value}${unit}`}
                width={56}
              />
              <Tooltip
                labelFormatter={(l) => new Date(l as string).toLocaleString()}
                formatter={(value) => [`${Number(value).toFixed(1)} ${unit}`, metricLabel]}
              />
              <Line
                type="monotone"
                dataKey="avg_value"
                name={metricLabel}
                stroke="hsl(var(--chart-1))"
                dot={false}
                strokeWidth={1.5}
              />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  )
}

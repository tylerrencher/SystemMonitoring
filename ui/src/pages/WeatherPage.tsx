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
import { ChartTimeRange, type TimeRange } from '@/components/ChartTimeRange'
import { fetchWeatherChart } from '@/lib/api'
import { makeXAxisFormatter } from '@/lib/utils'
import { subHours } from 'date-fns'

function defaultRange(): TimeRange {
  const to = new Date()
  return { from: subHours(to, 24).toISOString(), to: to.toISOString() }
}

function WeatherChart({
  metric,
  unit,
  label,
  range,
  color,
}: {
  metric: string
  unit: string
  label: string
  range: TimeRange
  color: string
}) {
  const { data = [], isFetching } = useQuery({
    queryKey: ['weather-chart', metric, range],
    queryFn: () => fetchWeatherChart({ metric, from: range.from, to: range.to }),
  })

  return (
    <div>
      <h2 className="mb-2 text-sm font-medium text-muted-foreground">{label}</h2>
      <div className="h-56">
        {isFetching ? (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            Loading…
          </div>
        ) : data.length === 0 ? (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            No data for selected range.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top: 4, right: 16, bottom: 4, left: 0 }}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis
                dataKey="time"
                tickFormatter={makeXAxisFormatter(range.from, range.to)}
                tick={{ fontSize: 11 }}
                minTickGap={60}
              />
              <YAxis
                tick={{ fontSize: 11 }}
                tickFormatter={(v: number) => `${v}${unit}`}
                width={48}
              />
              <Tooltip
                labelFormatter={(l) => new Date(l as string).toLocaleString()}
                formatter={(value) => [`${Number(value).toFixed(1)} ${unit}`, label]}
              />
              <Line
                type="monotone"
                dataKey="value"
                stroke={color}
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

export function WeatherPage() {
  const [range, setRange] = useState<TimeRange>(defaultRange)

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Weather</h1>
      <ChartTimeRange onChange={setRange} />
      <div className="space-y-8">
        <WeatherChart
          metric="temp_f"
          unit="°F"
          label="Temperature"
          range={range}
          color="hsl(var(--chart-1))"
        />
        <WeatherChart
          metric="humidity"
          unit="%"
          label="Humidity"
          range={range}
          color="hsl(var(--chart-2))"
        />
      </div>
    </div>
  )
}

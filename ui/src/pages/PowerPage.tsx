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
import { fetchSeries, fetchPowerChart } from '@/lib/api'
import { subHours } from 'date-fns'

function defaultRange(): TimeRange {
  const to = new Date()
  return { from: subHours(to, 24).toISOString(), to: to.toISOString() }
}

function fmtTime(iso: string) {
  const d = new Date(iso)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function PowerPage() {
  const [series, setSeries] = useState<string>('')
  const [range, setRange] = useState<TimeRange>(defaultRange)

  const { data: seriesList = [] } = useQuery({
    queryKey: ['series'],
    queryFn: fetchSeries,
  })

  const { data: points = [], isFetching } = useQuery({
    queryKey: ['power-chart', series, range],
    queryFn: () => fetchPowerChart({ series, from: range.from, to: range.to }),
    enabled: !!series,
  })

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Power</h1>

      <div className="flex flex-wrap items-center gap-4">
        <Select value={series} onValueChange={setSeries}>
          <SelectTrigger className="w-48">
            <SelectValue placeholder="Select series…" />
          </SelectTrigger>
          <SelectContent>
            {seriesList.map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <ChartTimeRange onChange={setRange} />
      </div>

      {!series && (
        <p className="text-sm text-muted-foreground">Select a series to view data.</p>
      )}

      {series && (
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
                  tickFormatter={fmtTime}
                  tick={{ fontSize: 11 }}
                  minTickGap={60}
                />
                <YAxis
                  tick={{ fontSize: 11 }}
                  tickFormatter={(v: number) => `${v}W`}
                  width={56}
                />
                <Tooltip
                  labelFormatter={(l) => new Date(l as string).toLocaleString()}
                  formatter={(value) => [`${Math.round(Number(value))} W`, 'Watts']}
                />
                <Line
                  type="monotone"
                  dataKey="avg_watts"
                  stroke="hsl(var(--primary))"
                  dot={false}
                  strokeWidth={1.5}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      )}
    </div>
  )
}

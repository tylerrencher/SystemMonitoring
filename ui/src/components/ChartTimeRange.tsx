import { useState } from 'react'
import { subHours } from 'date-fns'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Calendar } from '@/components/ui/calendar'
import type { DateRange } from 'react-day-picker'

export interface TimeRange {
  from: string
  to: string
}

interface ChartTimeRangeProps {
  onChange: (range: TimeRange) => void
}

const PRESETS = [
  { label: '1h', hours: 1 },
  { label: '6h', hours: 6 },
  { label: '24h', hours: 24 },
  { label: '7d', hours: 168 },
  { label: '30d', hours: 720 },
  { label: '90d', hours: 2160 },
]

export function ChartTimeRange({ onChange }: ChartTimeRangeProps) {
  const [active, setActive] = useState('24h')
  const [dateRange, setDateRange] = useState<DateRange | undefined>()

  const applyPreset = (label: string, hours: number) => {
    setActive(label)
    setDateRange(undefined)
    const to = new Date()
    const from = subHours(to, hours)
    onChange({ from: from.toISOString(), to: to.toISOString() })
  }

  const applyCustom = (range: DateRange | undefined) => {
    setDateRange(range)
    if (range?.from && range?.to) {
      setActive('custom')
      const to = new Date(range.to)
      to.setHours(23, 59, 59, 999)
      onChange({ from: range.from.toISOString(), to: to.toISOString() })
    }
  }

  const customLabel =
    active === 'custom' && dateRange?.from && dateRange?.to
      ? `${dateRange.from.toLocaleDateString()} – ${dateRange.to.toLocaleDateString()}`
      : 'Custom'

  return (
    <div className="flex flex-wrap items-center gap-2">
      {PRESETS.map(({ label, hours }) => (
        <Button
          key={label}
          size="sm"
          variant={active === label ? 'default' : 'outline'}
          onClick={() => applyPreset(label, hours)}
        >
          {label}
        </Button>
      ))}
      <Popover>
        <PopoverTrigger asChild>
          <Button size="sm" variant={active === 'custom' ? 'default' : 'outline'}>
            {customLabel}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="end">
          <Calendar mode="range" selected={dateRange} onSelect={applyCustom} numberOfMonths={2} />
        </PopoverContent>
      </Popover>
    </div>
  )
}

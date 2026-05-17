import { Card, CardContent } from '@/components/ui/card'

interface StatCardProps {
  label: string
  value: string | number
  unit?: string
  sub?: string
}

export function StatCard({ label, value, unit, sub }: StatCardProps) {
  return (
    <Card>
      <CardContent className="pb-4 pt-4">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</p>
        <p className="mt-1 text-2xl font-bold">
          {value}
          {unit && <span className="ml-1 text-sm font-normal text-muted-foreground">{unit}</span>}
        </p>
        {sub && <p className="mt-1 text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  )
}

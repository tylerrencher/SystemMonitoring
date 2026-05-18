export interface User {
  id: number
  name: string
  role: string
}

export interface Consumer {
  device: string
  series: string
  avg_watts: number
}

export interface WeatherSnapshot {
  temp_f?: number
  humidity?: number
}

export interface DashboardData {
  battery_soc_pct: number
  battery_power_w: number
  pv_power_w: number
  load_power_w: number
  generator_power_w: number
  power_source: string
  today_consumption_kwh: number
  top_consumers: Consumer[]
  weather?: WeatherSnapshot
}

export interface ChartPoint {
  time: string
  avg_watts: number | null
  max_watts?: number | null
  min_watts?: number | null
}

export interface SolarChartPoint {
  time: string
  avg_value: number | null
  max_value?: number | null
  min_value?: number | null
}

export interface WeatherChartPoint {
  time: string
  value: number | null
}

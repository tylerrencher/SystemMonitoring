import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function makeXAxisFormatter(from: string, to: string): (iso: string) => string {
  const spanDays = (new Date(to).getTime() - new Date(from).getTime()) / 86_400_000
  if (spanDays <= 1) {
    return (iso) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  if (spanDays <= 7) {
    return (iso) => new Date(iso).toLocaleString([], { month: 'short', day: 'numeric', hour: 'numeric' })
  }
  return (iso) => new Date(iso).toLocaleDateString([], { month: 'short', day: 'numeric' })
}

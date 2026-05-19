import { useState } from 'react'
import { Link, useRouterState, useNavigate } from '@tanstack/react-router'
import { LayoutDashboard, Zap, Sun, CloudRain, Bell, ChevronUp, LogOut, Menu, Moon, SunMedium, Monitor, LogIn } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetTrigger } from '@/components/ui/sheet'
import { Separator } from '@/components/ui/separator'
import { useAuth } from '@/lib/auth'
import { useTheme } from '@/lib/theme'
import { apiLogout } from '@/lib/api'

const NAV = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/power', label: 'Power', icon: Zap },
  { to: '/solar', label: 'Solar', icon: Sun },
  { to: '/weather', label: 'Weather', icon: CloudRain },
] as const

const AUTH_NAV = [
  { to: '/alerts', label: 'Alerts', icon: Bell },
] as const

function NavLink({ to, label, icon: Icon }: { to: string; label: string; icon: React.ElementType }) {
  const { location } = useRouterState()
  const isActive = location.pathname === to
  return (
    <Link
      to={to}
      className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
        isActive
          ? 'bg-accent text-accent-foreground'
          : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
      }`}
    >
      <Icon className="h-4 w-4" />
      {label}
    </Link>
  )
}

function ThemeCycleButton() {
  const { theme, setTheme } = useTheme()
  const next = theme === 'system' ? 'light' : theme === 'light' ? 'dark' : 'system'
  const Icon = theme === 'dark' ? Moon : theme === 'light' ? SunMedium : Monitor
  const label = theme === 'dark' ? 'Dark' : theme === 'light' ? 'Light' : 'System'
  return (
    <Button
      variant="ghost"
      size="sm"
      className="w-full justify-start gap-2 text-muted-foreground"
      onClick={() => setTheme(next)}
    >
      <Icon className="h-4 w-4" />
      {label}
    </Button>
  )
}

function NavContent({ onNavigate }: { onNavigate?: () => void }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [userOpen, setUserOpen] = useState(false)

  const handleLogout = async () => {
    await apiLogout()
    logout()
    void navigate({ to: '/login' })
  }

  return (
    <div className="flex h-full flex-col">
      <div className="px-4 py-5">
        <h2 className="text-base font-semibold tracking-tight">Energy Monitor</h2>
      </div>
      <Separator />
      <nav className="flex-1 space-y-1 px-2 py-4" onClick={onNavigate}>
        {NAV.map((item) => (
          <NavLink key={item.to} {...item} />
        ))}
        {user && AUTH_NAV.map((item) => (
          <NavLink key={item.to} {...item} />
        ))}
      </nav>
      <Separator />
      <div className="px-2 py-2">
        {user ? (
          <>
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-between px-3"
              onClick={() => setUserOpen((v) => !v)}
            >
              <span className="text-sm truncate">{user.name}</span>
              <ChevronUp className={`h-4 w-4 shrink-0 transition-transform ${userOpen ? '' : 'rotate-180'}`} />
            </Button>
            {userOpen && (
              <div className="mt-1 space-y-1 px-1">
                <p className="px-2 py-1 text-xs text-muted-foreground capitalize">{user.role}</p>
                <ThemeCycleButton />
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start gap-2 text-destructive hover:text-destructive"
                  onClick={handleLogout}
                >
                  <LogOut className="h-4 w-4" />
                  Log out
                </Button>
              </div>
            )}
          </>
        ) : (
          <div className="space-y-1 px-1">
            <ThemeCycleButton />
            <Link to="/login">
              <Button variant="ghost" size="sm" className="w-full justify-start gap-2 text-muted-foreground">
                <LogIn className="h-4 w-4" />
                Login
              </Button>
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}

export function AppSidebar() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <aside className="hidden lg:fixed lg:inset-y-0 lg:flex lg:w-64 lg:flex-col border-r bg-background">
        <NavContent />
      </aside>
      <div className="fixed top-0 left-0 right-0 h-14 z-40 flex items-center border-b bg-background px-4 lg:hidden">
        <Sheet open={open} onOpenChange={setOpen}>
          <SheetTrigger asChild>
            <Button variant="outline" size="icon">
              <Menu className="h-4 w-4" />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-64 p-0">
            <NavContent onNavigate={() => setOpen(false)} />
          </SheetContent>
        </Sheet>
      </div>
    </>
  )
}

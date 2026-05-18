import { createRootRouteWithContext, createRoute, createRouter, Outlet } from '@tanstack/react-router'
import type { AuthContextValue } from '@/lib/auth'
import { AppSidebar } from '@/components/AppSidebar'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { PowerPage } from '@/pages/PowerPage'
import { SolarPage } from '@/pages/SolarPage'
import { WeatherPage } from '@/pages/WeatherPage'

interface RouterContext {
  auth: AuthContextValue
}

function ProtectedLayout() {
  return (
    <div className="min-h-screen">
      <AppSidebar />
      <main className="pt-14 lg:pt-0 lg:pl-64">
        <div className="px-4 py-8 md:px-8">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: () => <Outlet />,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
})

const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: '_protected',
  component: ProtectedLayout,
})

const dashboardRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/',
  component: DashboardPage,
})

const powerRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/power',
  component: PowerPage,
})

const solarRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/solar',
  component: SolarPage,
})

const weatherRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: '/weather',
  component: WeatherPage,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  protectedRoute.addChildren([dashboardRoute, powerRoute, solarRoute, weatherRoute]),
])

export const router = createRouter({
  routeTree,
  context: { auth: undefined! },
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

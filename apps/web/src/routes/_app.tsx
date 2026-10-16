import { useQuery } from "@tanstack/react-query"
import { Outlet, createFileRoute, useLocation } from "@tanstack/react-router"

import { meQuery } from "@/api/queries"
import { Landing } from "@/components/landing"
import { AppShell, Onboarding } from "@/components/shell"
import { SignedOut } from "@/components/sign-in"
import { useSelectedWorkspace } from "@/lib/workspace"

export const Route = createFileRoute("/_app")({ component: AppLayout })

function AppLayout() {
  const { data: me } = useQuery(meQuery)
  const workspace = useSelectedWorkspace(me)
  const { pathname } = useLocation()
  if (!me) return pathname === "/" ? <Landing /> : <SignedOut />
  if (!workspace) return <Onboarding me={me} />
  return (
    <AppShell me={me} workspace={workspace}>
      <Outlet />
    </AppShell>
  )
}

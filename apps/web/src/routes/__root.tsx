import type { QueryClient } from "@tanstack/react-query"
import { Link, Outlet, createRootRouteWithContext } from "@tanstack/react-router"

import { configQuery, meQuery } from "@/api/queries"
import { CenterPanel } from "@/components/common"
import { Button } from "@/components/ui/button"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Toaster } from "@/components/ui/sonner"
import { Spinner } from "@/components/ui/spinner"

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  loader: ({ context }) =>
    Promise.all([context.queryClient.ensureQueryData(meQuery), context.queryClient.ensureQueryData(configQuery)]),
  pendingMs: 200,
  pendingMinMs: 300,
  pendingComponent: () => (
    <div className="grid min-h-svh place-items-center text-muted-foreground" role="status" aria-label="Loading">
      <Spinner className="size-5" />
    </div>
  ),
  errorComponent: ({ error }) => (
    <CenterPanel>
      <Card>
        <CardHeader>
          <CardTitle>Ricrac couldn't load</CardTitle>
          <CardDescription>{error instanceof Error ? error.message : "Unknown error"}</CardDescription>
        </CardHeader>
      </Card>
    </CenterPanel>
  ),
  component: () => (
    <>
      <Outlet />
      <Toaster position="bottom-right" />
    </>
  ),
  notFoundComponent: () => (
    <CenterPanel>
      <Card>
        <CardHeader>
          <CardTitle>Page not found</CardTitle>
          <CardDescription>There's nothing at this address.</CardDescription>
        </CardHeader>
        <div className="px-6">
          <Button asChild variant="outline" className="w-full">
            <Link to="/">Go to the dashboard</Link>
          </Button>
        </div>
      </Card>
    </CenterPanel>
  ),
})

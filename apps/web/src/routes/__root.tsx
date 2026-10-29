import type { QueryClient } from "@tanstack/react-query";
import {
  createRootRouteWithContext,
  type ErrorComponentProps,
  Outlet,
  type ParsedLocation,
  redirect,
  useLocation,
} from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { TriangleAlertIcon } from "@/components/icons";
import NotFoundTemplate from "@/components/navigation/not-found-template";
import Providers from "@/components/providers/providers";
import { meQuery } from "@/lib/queries/me";

export type RouterContext = {
  queryClient: QueryClient;
};

const PUBLIC_PATHS = new Set(["/", "/sign-in", "/sign-in/email", "/privacy", "/terms"]);

/** Paths outside workspaces that need signing in; workspaces decide for themselves. */
const ACCOUNT_PATHS = ["create-workspace", "oauth"];

function isPublicPath(pathname: string) {
  const normalized = pathname.length > 1 ? pathname.replace(/\/$/, "") : pathname;
  return PUBLIC_PATHS.has(normalized);
}

function isAccountPath(pathname: string) {
  return ACCOUNT_PATHS.includes(pathname.split("/")[1] ?? "");
}

export const Route = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async ({ context, location }) => {
    if (isPublicPath(location.pathname)) return;
    const me = await context.queryClient.ensureQueryData(meQuery);
    if (!me && isAccountPath(location.pathname)) {
      throw redirect({ to: "/sign-in", search: { redirect: location.href } });
    }
    return { me };
  },
  component: RootComponent,
  notFoundComponent: () => (
    <main className="flex min-h-svh flex-col">
      <NotFoundTemplate
        code={404}
        description="Not found."
        buttonText="Go home"
        buttonTo="/"
        Icon={TriangleAlertIcon}
      />
    </main>
  ),
  errorComponent: RootError,
});

let retriedLocation: ParsedLocation | undefined;

// TanStack/router#8198: a page being left can render once more after its route is gone and throw.
function RootError({ error, reset }: ErrorComponentProps) {
  const location = useLocation();
  const shouldRetry = retriedLocation !== location;
  useLayoutEffect(() => {
    if (!shouldRetry) return;
    retriedLocation = location;
    reset();
  }, [shouldRetry, location, reset]);
  if (shouldRetry) return null;
  return (
    <main className="flex min-h-svh flex-col">
      <NotFoundTemplate
        code={500}
        description={error instanceof Error ? error.message : "Something went wrong."}
        buttonText="Go home"
        buttonTo="/"
        Icon={TriangleAlertIcon}
      />
    </main>
  );
}

function RootComponent() {
  return (
    <Providers>
      <Outlet />
    </Providers>
  );
}

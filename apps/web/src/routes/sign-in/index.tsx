import { createFileRoute, Link, redirect } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { SignInError, SignInOptions } from "@/components/auth/sign-in";
import { AuthShell } from "@/components/layout/auth-shell";
import { appConfigQuery, meQuery, safeRedirect } from "@/lib/queries/me";

const searchSchema = z.object({
  redirect: z.string().optional(),
  signin_error: z.string().optional(),
});

export const Route = createFileRoute("/sign-in/")({
  validateSearch: zodValidator(searchSchema),
  beforeLoad: async ({ context, search }) => {
    const me = await context.queryClient.ensureQueryData(meQuery);
    if (!me) return;
    throw redirect({ href: safeRedirect(search.redirect) ?? "/" });
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(appConfigQuery),
  component: SignIn,
});

function SignIn() {
  const search = Route.useSearch();
  const redirectTo = safeRedirect(search.redirect) ?? undefined;
  const isConnectingApp = redirectTo?.startsWith("/oauth/authorize") ?? false;
  return (
    <AuthShell
      title={isConnectingApp ? "Sign in to connect an app" : "Sign in to Ricrac"}
      description={
        isConnectingApp
          ? "An app wants to work in your Ricrac workspaces. Sign in first."
          : "The issue tracker for people and agents."
      }
      footer={
        <p className="text-xs">
          By continuing you agree to the{" "}
          <Link to="/terms" className="underline underline-offset-4">
            terms
          </Link>{" "}
          and the{" "}
          <Link to="/privacy" className="underline underline-offset-4">
            privacy policy
          </Link>
          .
        </p>
      }
    >
      <div className="flex w-full flex-col gap-4">
        <SignInError reason={search.signin_error} />
        <SignInOptions redirect={redirectTo} />
      </div>
    </AuthShell>
  );
}

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { GoogleIcon } from "@/components/icons/google";
import { ErrorCard } from "@/components/layout/error-card";
import { Button } from "@/components/ui/button";
import { appConfigQuery, signInHref } from "@/lib/queries/me";

const signInErrors: Record<string, string> = {
  expired: "The sign-in took too long or was interrupted. Try again.",
  denied: "Sign-in was canceled in Google.",
  failed: "Signing in with Google didn't work. Try again.",
  unverified: "Your Google account's email address isn't verified yet.",
  not_configured: "Sign-in with Google isn't set up on this server yet.",
};

export function GoogleSignInButton({ redirect }: { redirect?: string }) {
  const [isLeaving, setIsLeaving] = useState(false);
  return (
    <Button
      variant="outline"
      className="w-full"
      isPending={isLeaving}
      render={<a href={signInHref(redirect)} onClick={() => setIsLeaving(true)} />}
    >
      <GoogleIcon className="size-4" />
      Continue with Google
    </Button>
  );
}

export function SignInError({ reason }: { reason?: string }) {
  if (!reason) return null;
  return <ErrorCard title={signInErrors[reason] ?? signInErrors.failed} />;
}

export function GoogleNotConfigured() {
  const { data: config } = useQuery(appConfigQuery);
  if (!config || config.googleConfigured) return null;
  return (
    <p className="bg-muted text-muted-foreground rounded-lg p-4 text-sm">
      Google sign-in isn't configured. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET on the server
      to let people sign in; API keys and OAuth tokens keep working without it.
    </p>
  );
}

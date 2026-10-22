import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { z } from "zod";

import { GoogleIcon } from "@/components/icons/google";
import { MailIcon } from "@/components/icons/symbols.gen";
import { ErrorCard } from "@/components/layout/error-card";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { appConfigQuery, signInHref, useSendSignInLinkMutation } from "@/lib/queries/me";

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

export function SignInNotConfigured() {
  const { data: config } = useQuery(appConfigQuery);
  if (!config || config.googleConfigured || config.emailConfigured) return null;
  return (
    <p className="bg-muted text-muted-foreground rounded-lg p-4 text-sm">
      Sign-in isn't configured. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET, or SMTP_URL and
      EMAIL_FROM for sign-in links, on the server to let people sign in; API keys and OAuth tokens
      keep working without them.
    </p>
  );
}

const emailSchema = z.object({ email: z.string().trim().email("Enter a valid email address.") });

export function EmailSignInForm({ redirect }: { redirect?: string }) {
  const [sentTo, setSentTo] = useState<string | null>(null);
  const { mutateAsync: sendLink } = useSendSignInLinkMutation();
  const form = useAppForm({
    defaultValues: { email: "" },
    validators: { onSubmit: emailSchema },
    onSubmit: async ({ value }) => {
      const { email } = emailSchema.parse(value);
      await sendLink({ email, returnTo: redirect });
      setSentTo(email.toLowerCase());
    },
  });

  if (sentTo) {
    return (
      <div className="bg-muted flex flex-col items-center gap-3 rounded-lg p-4 text-center">
        <MailIcon className="text-muted-foreground size-5" />
        <p className="text-sm">
          Check your email. We sent a sign-in link to <span className="font-medium">{sentTo}</span>.
        </p>
        <Button variant="ghost" size="sm" onClick={() => setSentTo(null)}>
          Use another email
        </Button>
      </div>
    );
  }
  return (
    <form className="flex flex-col gap-2" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="email"
        children={(field) => (
          <field.TextField
            label="Email"
            hideLabel
            type="email"
            autoComplete="email"
            placeholder="name@example.com"
          />
        )}
      />
      <form.AppForm>
        <form.SubmitButton className="w-full">Continue with email</form.SubmitButton>
      </form.AppForm>
    </form>
  );
}

export function SignInOptions({ redirect }: { redirect?: string }) {
  const { data: config } = useQuery(appConfigQuery);
  const hasGoogle = config?.googleConfigured !== false;
  const hasEmail = config?.emailConfigured ?? false;
  return (
    <div className="flex w-full flex-col gap-4">
      <SignInNotConfigured />
      {hasGoogle && <GoogleSignInButton redirect={redirect} />}
      {hasGoogle && hasEmail && (
        <div className="text-muted-foreground flex items-center gap-2 text-xs">
          <Separator className="flex-1" />
          or
          <Separator className="flex-1" />
        </div>
      )}
      {hasEmail && <EmailSignInForm redirect={redirect} />}
    </div>
  );
}

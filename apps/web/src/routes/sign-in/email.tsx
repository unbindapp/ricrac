import { createFileRoute, Link } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { AuthShell } from "@/components/layout/auth-shell";
import { Button } from "@/components/ui/button";
import { useFinishSignInLinkMutation } from "@/lib/queries/me";

const searchSchema = z.object({
  email: z.string().catch(""),
  token: z.string().catch(""),
});

export const Route = createFileRoute("/sign-in/email")({
  validateSearch: zodValidator(searchSchema),
  component: SignInWithLink,
});

function SignInWithLink() {
  const { email, token } = Route.useSearch();
  const { mutate: finish, isPending, isSuccess } = useFinishSignInLinkMutation();
  return (
    <AuthShell
      title="Sign in to Ricrac"
      description={email ? `Continue as ${email}.` : "This sign-in link is incomplete."}
      footer={
        <Link to="/sign-in" className="text-xs underline underline-offset-4">
          Ask for a new link
        </Link>
      }
    >
      <Button
        className="w-full"
        disabled={!email || !token}
        isPending={isPending || isSuccess}
        onClick={() => finish({ email, token })}
      >
        Sign in
      </Button>
    </AuthShell>
  );
}

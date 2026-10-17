import { TriangleAlertIcon, XIcon } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { githubErrorMessage } from "@/lib/helpers/github-sync";

const PARAM = "github_error";

/** Why GitHub sent you back without connecting, read once from the URL and then dropped from it. */
function useGithubErrorParam() {
  const [reason, setReason] = useState(() =>
    new URLSearchParams(window.location.search).get(PARAM),
  );
  useEffect(() => {
    const url = new URL(window.location.href);
    if (!url.searchParams.has(PARAM)) return;
    url.searchParams.delete(PARAM);
    window.history.replaceState(window.history.state, "", url);
  }, []);
  return { reason, dismiss: () => setReason(null) };
}

export function GithubErrorNotice({ title }: { title: string }) {
  const { reason, dismiss } = useGithubErrorParam();
  if (!reason) return null;
  return (
    <div
      role="alert"
      className="border-warning/5-10 bg-warning/1-10 flex w-full items-start gap-2 rounded-lg border p-4"
    >
      <TriangleAlertIcon className="text-warning mt-0.5 size-4 shrink-0" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="text-sm font-medium">{title}</p>
        <p className="text-muted-foreground text-sm">{githubErrorMessage(reason)}</p>
      </div>
      <Button variant="ghost" size="icon" aria-label="Dismiss" onClick={dismiss}>
        <XIcon className="size-4" />
      </Button>
    </div>
  );
}

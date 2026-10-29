import { useQuery } from "@tanstack/react-query";
import { useLocation } from "@tanstack/react-router";
import { ReactNode } from "react";

import { CircleDotIcon } from "@/components/icons";
import { LinkButton } from "@/components/ui/button";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";

/** Where visitors read a public workspace's issues: no navigation to what only members see. */
export function VisitorShell({ children }: { children: ReactNode }) {
  const { workspace } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  const { href } = useLocation();

  return (
    <div
      data-visitor
      className="group/shell bg-sidebar dank:md:gap-3 dank:md:p-3 flex h-svh w-full flex-col overflow-hidden md:gap-2 md:p-2"
    >
      <header className="flex h-11 shrink-0 items-center gap-2 px-4 md:px-2">
        <WorkspaceIcon name={workspace.name} logoUrl={workspace.logoUrl} />
        <span className="min-w-0 truncate text-sm font-medium">{workspace.name}</span>
        <LinkButton
          to="/$workspace/issues"
          params={{ workspace: workspace.urlKey }}
          variant="ghost"
          size="sm"
        >
          <CircleDotIcon className="size-4" />
          Issues
        </LinkButton>
        <div className="flex-1" />
        {!me && (
          <LinkButton to="/sign-in" search={{ redirect: href }} size="sm">
            Sign in
          </LinkButton>
        )}
      </header>
      <main className="bg-background dank:border-0 flex min-h-0 flex-1 flex-col overflow-hidden md:rounded-lg md:border">
        {children}
      </main>
    </div>
  );
}

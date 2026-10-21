import { SearchIcon } from "lucide-react";
import { ReactNode } from "react";

import { CommandPanel } from "@/components/command-panel/command-panel";
import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { NewIssueDialog } from "@/components/issues/new-issue-dialog";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { SidebarButton } from "@/components/layout/sidebar-button";
import { NewProjectDialog } from "@/components/projects/new-project-dialog";
import { CreateTeamDialog } from "@/components/settings/create-team-dialog";
import { InviteMemberDialog } from "@/components/settings/invite-member-dialog";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Sidebar } from "@/components/workspace/sidebar";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";
import { SIDEBAR_HOTKEY, useSidebarStore } from "@/lib/stores/sidebar";

export function WorkspaceShell({ children }: { children: ReactNode }) {
  const { canEdit } = useWorkspace();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const isSidebarCollapsed = useSidebarStore((state) => state.isCollapsed);
  const toggleSidebar = useSidebarStore((state) => state.toggle);

  useHotkeyScope([
    { hotkey: "C", callback: () => openNewIssue(), enabled: canEdit },
    { hotkey: SIDEBAR_HOTKEY, callback: toggleSidebar },
  ]);

  return (
    <div
      data-sidebar-collapsed={isSidebarCollapsed || undefined}
      className="group/shell bg-sidebar flex h-svh w-full overflow-hidden"
    >
      {!isSidebarCollapsed && (
        <aside className="hidden w-60 shrink-0 md:flex">
          <Sidebar />
        </aside>
      )}
      <div className="flex min-w-0 flex-1 flex-col md:py-2 md:pr-2 md:group-data-sidebar-collapsed/shell:pl-2">
        <FallbackMobileBar />
        <main className="bg-background flex min-h-0 flex-1 flex-col overflow-hidden md:rounded-lg md:border">
          {children}
        </main>
      </div>
      <MobileNavDrawer />
      <CommandPanel />
      <NewIssueDialog />
      <NewProjectDialog />
      {canEdit && <CreateTeamDialog />}
      {canEdit && <InviteMemberDialog />}
    </div>
  );
}

function MobileNavDrawer() {
  const isOpen = useMobileNavStore((state) => state.isOpen);
  const setOpen = useMobileNavStore((state) => state.setOpen);
  return (
    <Drawer direction="left" open={isOpen} onOpenChange={setOpen}>
      <DrawerContent className="w-72 max-w-full">
        <DrawerTitle className="sr-only">Navigation</DrawerTitle>
        <Sidebar onNavigate={() => setOpen(false)} />
      </DrawerContent>
    </Drawer>
  );
}

function FallbackMobileBar() {
  const { workspace } = useWorkspace();
  const openCommandPanel = useCommandPanelStore((state) => state.setOpen);
  return (
    <div className="flex h-11 shrink-0 items-center gap-2 border-b px-4 group-has-data-mobile-bar/shell:hidden md:hidden">
      <SidebarButton />
      <WorkspaceIcon name={workspace.name} logoUrl={workspace.logoUrl} />
      <span className="min-w-0 flex-1 truncate text-sm font-medium">{workspace.name}</span>
      <Button
        variant="ghost"
        size="icon"
        aria-label="Search"
        onClick={() => openCommandPanel(true)}
      >
        <SearchIcon className="size-4" />
      </Button>
    </div>
  );
}

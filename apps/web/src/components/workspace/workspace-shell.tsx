import { ReactNode } from "react";

import { AssistantPanel } from "@/components/assistant/assistant-panel";
import { CommandPanel } from "@/components/command-panel/command-panel";
import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { SearchIcon } from "@/components/icons";
import { NewIssueDialog } from "@/components/issues/new-issue-dialog";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { SidebarButton } from "@/components/layout/sidebar-button";
import { NewProjectDialog } from "@/components/projects/new-project-dialog";
import { InviteMemberDialog } from "@/components/settings/invite-member-dialog";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";
import { useUiTheme } from "@/themes/ui-theme-provider";

export function WorkspaceShell({ children }: { children: ReactNode }) {
  const { canEdit } = useWorkspace();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const { Navigation, TopBar, SettingsWindow, useIsNavigationHidden } = useUiTheme();
  const isNavigationHidden = useIsNavigationHidden();

  useHotkeyScope([{ hotkey: "C", callback: () => openNewIssue(), enabled: canEdit }]);

  return (
    <div
      data-sidebar-collapsed={isNavigationHidden || undefined}
      className="group/shell bg-sidebar dank:md:gap-3 dank:md:p-3 flex h-svh w-full overflow-hidden"
    >
      <Navigation />
      <div className="dank:md:gap-3 dank:md:p-0 flex min-w-0 flex-1 flex-col md:gap-2 md:py-2 md:pr-2 md:group-data-sidebar-collapsed/shell:pl-2">
        <FallbackMobileBar />
        {TopBar && <TopBar />}
        <div className="dank:gap-3 flex min-h-0 flex-1 gap-2">
          <main className="bg-background dank:border-0 flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden has-data-[slot=split-view]:border-0 has-data-[slot=split-view]:bg-transparent md:rounded-lg md:border">
            {children}
          </main>
          <AssistantPanel />
        </div>
      </div>
      <MobileNavDrawer />
      {SettingsWindow && <SettingsWindow />}
      <CommandPanel />
      <NewIssueDialog />
      <NewProjectDialog />
      {canEdit && <InviteMemberDialog />}
    </div>
  );
}

function MobileNavDrawer() {
  const { NavigationDrawer } = useUiTheme();
  const isOpen = useMobileNavStore((state) => state.isOpen);
  const setOpen = useMobileNavStore((state) => state.setOpen);
  return (
    <Drawer direction="left" open={isOpen} onOpenChange={setOpen}>
      <DrawerContent className="dank:w-80 w-72 max-w-full">
        <DrawerTitle className="sr-only">Navigation</DrawerTitle>
        <NavigationDrawer onNavigate={() => setOpen(false)} />
      </DrawerContent>
    </Drawer>
  );
}

function FallbackMobileBar() {
  const { workspace } = useWorkspace();
  const openCommandPanel = useCommandPanelStore((state) => state.setOpen);
  return (
    <div className="dank:h-16 dank:border-0 dank:px-1 flex h-11 shrink-0 items-center gap-2 border-b px-4 group-has-data-mobile-bar/shell:hidden md:hidden">
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

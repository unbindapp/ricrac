import { useQuery } from "@tanstack/react-query";
import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { useTheme } from "next-themes";
import { ReactNode, useState } from "react";

import { useIsAssistantAvailable } from "@/components/assistant/use-assistant-available";
import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { ShortcutKeys } from "@/components/command-panel/shortcut-keys";
import { ShortcutsDialog } from "@/components/command-panel/shortcuts-dialog";
import {
  BoxIcon,
  CheckIcon,
  CircleDotIcon,
  DiamondIcon,
  FileTextIcon,
  InboxIcon,
  KeyboardIcon,
  LayersIcon,
  ListTodoIcon,
  LogOutIcon,
  MessagesSquareIcon,
  MonitorIcon,
  MoonIcon,
  RefreshCwIcon,
  SettingsIcon,
  SparklesIcon,
  SquarePenIcon,
  SunIcon,
  TargetIcon,
  TriangleAlertIcon,
  TriangleIcon,
  UserCircleIcon,
  UserPlusIcon,
} from "@/components/icons";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import type { TTheme } from "@/components/providers/themes";
import { SearchHitGroups } from "@/components/search/search-hit-groups";
import { useSearch } from "@/components/search/use-search";
import { useInviteStore } from "@/components/settings/invite-store";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { GO_TO_KEYS } from "@/lib/hotkeys/shortcuts";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useHotkeySequenceScope } from "@/lib/hotkeys/use-hotkey-sequences";
import { meQuery, signOut } from "@/lib/queries/me";
import { useAssistantStore } from "@/lib/stores/assistant";

type TShortcutDestination = keyof typeof GO_TO_KEYS;
type TDestination = TShortcutDestination | "cycles" | "milestones";

const shortcutDestinations = Object.keys(GO_TO_KEYS) as TShortcutDestination[];

function isShortcutDestination(destination: TDestination): destination is TShortcutDestination {
  return destination in GO_TO_KEYS;
}

/** The pages G then a letter, and the panel's Go to group, lead to. */
function useDestinations(): Record<
  TDestination,
  { label: string; icon: ReactNode; link: LinkProps; isHidden?: boolean }
> {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const team = useWorkspaceTeam();
  const params = { workspace: workspace.urlKey };
  return {
    inbox: { label: "Inbox", icon: <InboxIcon />, link: { to: "/$workspace/inbox", params } },
    myIssues: {
      label: "My issues",
      icon: <UserCircleIcon />,
      link: { to: "/$workspace/my-issues", params },
    },
    issues: {
      label: "Issues",
      icon: <CircleDotIcon />,
      link: { to: "/$workspace/issues", params },
    },
    backlog: {
      label: "Backlog",
      icon: <ListTodoIcon />,
      link: { to: "/$workspace/issues", params, search: { tab: "backlog" } },
    },
    triage: {
      label: "Triage",
      icon: <TriangleIcon />,
      link: { to: "/$workspace/triage", params },
      isHidden: !team?.triageEnabled,
    },
    cycles: {
      label: "Cycles",
      icon: <RefreshCwIcon />,
      link: { to: "/$workspace/cycles", params },
      isHidden: !team?.cyclesEnabled,
    },
    projects: {
      label: "Projects",
      icon: <BoxIcon />,
      link: { to: "/$workspace/projects", params },
    },
    milestones: {
      label: "Milestones",
      icon: <DiamondIcon />,
      link: { to: "/$workspace/milestones", params },
    },
    views: { label: "Views", icon: <LayersIcon />, link: { to: "/$workspace/views", params } },
    initiatives: {
      label: "Initiatives",
      icon: <TargetIcon />,
      link: { to: "/$workspace/initiatives", params },
      isHidden: !canEdit,
    },
    documents: {
      label: "Documents",
      icon: <FileTextIcon />,
      link: { to: "/$workspace/documents", params },
    },
    forum: {
      label: "Forum",
      icon: <MessagesSquareIcon />,
      link: { to: "/$workspace/forum", params },
    },
    settings: {
      label: "Settings",
      icon: <SettingsIcon />,
      link: { to: isAdmin ? "/$workspace/settings" : "/$workspace/settings/account", params },
    },
  };
}

/** The command panel (Mod+K or /), the shortcuts help (?) and G-then-letter navigation. */
export function CommandPanel() {
  const isOpen = useCommandPanelStore((state) => state.isOpen);
  const setOpen = useCommandPanelStore((state) => state.setOpen);
  const setShortcutsOpen = useCommandPanelStore((state) => state.setShortcutsOpen);
  const navigate = useNavigate();
  const destinations = useDestinations();

  useHotkeyScope([
    { hotkey: "Mod+K", callback: () => setOpen(true), ignoreInputs: false },
    { hotkey: "/", callback: () => setOpen(true) },
    { hotkey: "Shift+/", callback: () => setShortcutsOpen(true) },
  ]);
  useHotkeyScope([{ hotkey: "Mod+K", callback: () => setOpen(false), ignoreInputs: false }], {
    enabled: isOpen,
    modal: true,
  });
  useHotkeySequenceScope(
    shortcutDestinations.map((destination) => ({
      sequence: ["G", GO_TO_KEYS[destination]],
      callback: () => void navigate(destinations[destination].link),
    })),
  );

  return (
    <>
      <Dialog open={isOpen} onOpenChange={setOpen}>
        <DialogContent
          variant="styleless"
          hideXButton
          animate={false}
          className="mt-16 mb-auto w-2xl max-w-full self-start sm:mt-24"
        >
          <DialogTitle className="sr-only">Command panel</DialogTitle>
          {isOpen && <CommandPanelContent onDone={() => setOpen(false)} />}
        </DialogContent>
      </Dialog>
      <ShortcutsDialog />
    </>
  );
}

type TAction = {
  id: string;
  label: string;
  icon: ReactNode;
  keywords?: string[];
  shortcut?: { keys: string[]; then?: boolean };
  isActive?: boolean;
  run: () => void;
};

function CommandPanelContent({ onDone }: { onDone: () => void }) {
  const [query, setQuery] = useState("");
  const navigate = useNavigate();
  const { theme: currentTheme, setTheme } = useTheme();
  const { workspace, canEdit } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  const openNewIssue = useNewIssueStore((state) => state.open);
  const openNewProject = useNewProjectStore((state) => state.open);
  const openInvite = useInviteStore((state) => state.setOpen);
  const setShortcutsOpen = useCommandPanelStore((state) => state.setShortcutsOpen);
  const isAssistantAvailable = useIsAssistantAvailable();
  const openAssistant = useAssistantStore((state) => state.setOpen);
  const destinations = useDestinations();
  const search = useSearch(query);

  const go = (link: LinkProps) => {
    onDone();
    void navigate(link);
  };
  const run = (fn: () => void) => () => {
    onDone();
    fn();
  };

  const create: TAction[] = canEdit
    ? [
        {
          id: "new-issue",
          label: "New issue",
          icon: <SquarePenIcon />,
          keywords: ["create", "add"],
          shortcut: { keys: ["C"] },
          run: run(() => openNewIssue()),
        },
        {
          id: "new-project",
          label: "New project",
          icon: <BoxIcon />,
          keywords: ["create", "add"],
          run: run(() => openNewProject()),
        },
        {
          id: "invite",
          label: "Invite people",
          icon: <UserPlusIcon />,
          keywords: ["member", "teammate", "add", "email"],
          run: run(() => openInvite(true)),
        },
      ]
    : [];

  const assistant: TAction[] = isAssistantAvailable
    ? [
        {
          id: "ask-claude",
          label: "Ask Claude",
          icon: <SparklesIcon />,
          keywords: ["assistant", "ai", "chat", "question"],
          run: run(() => openAssistant(true)),
        },
      ]
    : [];

  const goTo: TAction[] = (Object.keys(destinations) as TDestination[])
    .filter((destination) => !destinations[destination].isHidden)
    .map((destination) => {
      const { label, icon, link } = destinations[destination];
      const key = isShortcutDestination(destination) ? GO_TO_KEYS[destination] : undefined;
      return {
        id: `go-${destination}`,
        label: `Go to ${label.toLowerCase()}`,
        icon,
        keywords: ["navigate", "open"],
        shortcut: key ? { keys: ["G", key], then: true } : undefined,
        run: () => go(link),
      };
    });

  const workspaces: TAction[] = (me?.workspaces ?? [])
    .filter((item) => item.id !== workspace.id)
    .map((item) => ({
      id: `workspace-${item.id}`,
      label: `Switch to ${item.name}`,
      keywords: [item.urlKey, "workspace"],
      icon: <WorkspaceIcon name={item.name} logoUrl={item.logoUrl} />,
      run: () => go({ to: "/$workspace", params: { workspace: item.urlKey } }),
    }));

  const themeAction = (theme: TTheme, label: string, icon: ReactNode): TAction => ({
    id: `theme-${theme}`,
    label,
    icon,
    keywords: ["theme", "appearance", "mode"],
    isActive: currentTheme === theme,
    run: run(() => setTheme(theme)),
  });
  const preferences: TAction[] = [
    themeAction("light", "Light theme", <SunIcon />),
    themeAction("dark", "Dark theme", <MoonIcon />),
    themeAction("system", "System theme", <MonitorIcon />),
    {
      id: "shortcuts",
      label: "Keyboard shortcuts",
      icon: <KeyboardIcon />,
      keywords: ["hotkeys", "keys", "help"],
      shortcut: { keys: ["?"] },
      run: run(() => setShortcutsOpen(true)),
    },
    {
      id: "sign-out",
      label: "Sign out",
      icon: <LogOutIcon />,
      keywords: ["log out", "logout"],
      run: run(() => void signOut()),
    },
  ];

  return (
    <Command loop variant="searchView" className="h-auto sm:max-h-120">
      <CommandInput
        autoFocus
        value={query}
        onValueChange={setQuery}
        showSpinner={search.isPending}
        placeholder="Search issues, projects, documents, discussions… or type a command"
      />
      <ScrollArea className="min-h-0">
        <CommandList>
          <CommandEmpty>{search.isPending ? "Searching…" : "Nothing found"}</CommandEmpty>
          {search.error && (
            <div
              role="alert"
              className="text-destructive flex items-center gap-2 px-3 py-2 text-sm"
            >
              <TriangleAlertIcon className="size-4" />
              Search failed: {search.error.message}
            </div>
          )}
          <SearchHitGroups hits={search.hits} query={query} onOpen={go} />
          <ActionGroup heading="Create" actions={create} />
          <ActionGroup heading="Claude" actions={assistant} />
          <ActionGroup heading="Go to" actions={goTo} />
          <ActionGroup heading="Workspaces" actions={workspaces} />
          <ActionGroup heading="Preferences" actions={preferences} />
        </CommandList>
      </ScrollArea>
    </Command>
  );
}

function ActionGroup({ heading, actions }: { heading: string; actions: TAction[] }) {
  if (actions.length === 0) return null;
  return (
    <CommandGroup heading={heading}>
      {actions.map((action) => (
        <CommandItem
          key={action.id}
          value={action.id}
          keywords={[action.label, ...(action.keywords ?? [])]}
          onSelect={action.run}
          className="[&>svg]:text-muted-foreground [&>svg]:size-4"
        >
          {action.icon}
          <span className="min-w-0 truncate">{action.label}</span>
          {action.isActive && <CheckIcon aria-label="Current" className="ml-auto" />}
          {action.shortcut && (
            <span className="ml-auto">
              <ShortcutKeys keys={action.shortcut.keys} then={action.shortcut.then} />
            </span>
          )}
        </CommandItem>
      ))}
    </CommandGroup>
  );
}

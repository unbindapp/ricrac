/** A shortcut as the help lists it: keys pressed together, or one after the other. */
export type TShortcut = { label: string; keys: string[]; then?: boolean };

export type TShortcutGroup = { title: string; shortcuts: TShortcut[] };

/** Where G then a letter goes. */
export const GO_TO_KEYS = {
  inbox: "I",
  myIssues: "M",
  issues: "A",
  projects: "P",
  views: "V",
  initiatives: "N",
  documents: "D",
  settings: "S",
} as const;

export const shortcutGroups: TShortcutGroup[] = [
  {
    title: "General",
    shortcuts: [
      { label: "Open command panel", keys: ["Mod+K"] },
      { label: "Search", keys: ["/"] },
      { label: "New issue", keys: ["C"] },
      { label: "Keyboard shortcuts", keys: ["?"] },
    ],
  },
  {
    title: "Go to",
    shortcuts: [
      { label: "Inbox", keys: ["G", GO_TO_KEYS.inbox], then: true },
      { label: "My issues", keys: ["G", GO_TO_KEYS.myIssues], then: true },
      { label: "All issues", keys: ["G", GO_TO_KEYS.issues], then: true },
      { label: "Projects", keys: ["G", GO_TO_KEYS.projects], then: true },
      { label: "Views", keys: ["G", GO_TO_KEYS.views], then: true },
      { label: "Initiatives", keys: ["G", GO_TO_KEYS.initiatives], then: true },
      { label: "Documents", keys: ["G", GO_TO_KEYS.documents], then: true },
      { label: "Settings", keys: ["G", GO_TO_KEYS.settings], then: true },
    ],
  },
  {
    title: "Issue lists",
    shortcuts: [
      { label: "Next / previous", keys: ["J", "K"] },
      { label: "Open", keys: ["O"] },
      { label: "Select", keys: ["X"] },
      { label: "Clear the selection", keys: ["Escape"] },
    ],
  },
  {
    title: "Issue",
    shortcuts: [
      { label: "Status", keys: ["S"] },
      { label: "Priority", keys: ["P"] },
      { label: "Assignee", keys: ["A"] },
      { label: "Labels", keys: ["L"] },
      { label: "Project", keys: ["Shift+P"] },
      { label: "Cycle", keys: ["Shift+C"] },
      { label: "Estimate", keys: ["E"] },
      { label: "Due date", keys: ["D"] },
    ],
  },
  {
    title: "Inbox",
    shortcuts: [
      { label: "Next / previous", keys: ["J", "K"] },
      { label: "Open", keys: ["Enter"] },
      { label: "Archive or move back", keys: ["E"] },
      { label: "Mark read or unread", keys: ["U"] },
      { label: "Snooze", keys: ["H"] },
      { label: "Close", keys: ["Escape"] },
    ],
  },
];

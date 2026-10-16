import { createContext, useContext, useSyncExternalStore } from "react"

import type { Me } from "@/api/types/Me"
import type { WorkspaceSummary } from "@/api/types/WorkspaceSummary"

const KEY = "ricrac:workspace"
const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  window.addEventListener("storage", listener)
  return () => {
    listeners.delete(listener)
    window.removeEventListener("storage", listener)
  }
}

export function selectWorkspace(id: string) {
  localStorage.setItem(KEY, id)
  listeners.forEach((listener) => listener())
}

/** The workspace picked in the switcher, or the first one. */
export function useSelectedWorkspace(me: Me | null | undefined): WorkspaceSummary | null {
  const stored = useSyncExternalStore(subscribe, () => localStorage.getItem(KEY))
  if (!me) return null
  return me.workspaces.find((w) => w.id === stored) ?? me.workspaces[0] ?? null
}

export const isAdmin = (role: string) => role === "owner" || role === "admin"

export const WorkspaceContext = createContext<WorkspaceSummary | null>(null)

/** The workspace selected in the switcher. Only inside the app shell. */
export function useWorkspace() {
  const workspace = useContext(WorkspaceContext)
  if (!workspace) throw new Error("useWorkspace outside the app shell")
  return workspace
}

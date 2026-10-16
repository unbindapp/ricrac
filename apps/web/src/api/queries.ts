import { queryOptions } from "@tanstack/react-query"

import { api, op } from "./client"
import type { ApiKey } from "./types/ApiKey"
import type { AppConfig } from "./types/AppConfig"
import type { ConnectedApp } from "./types/ConnectedApp"
import type { Me } from "./types/Me"
import type { OAuthApp } from "./types/OAuthApp"
import type {
  CustomView,
  Document,
  Initiative,
  Invitation,
  Issue,
  IssueSummary,
  Member,
  Page,
  Project,
  Workspace,
} from "./v1"

/** The signed-in user, or null for visitors. */
export const meQuery = queryOptions({
  queryKey: ["me"],
  queryFn: () => api<Me | null>("GET", "/me"),
  staleTime: Infinity,
})

export const configQuery = queryOptions({
  queryKey: ["config"],
  queryFn: () => api<AppConfig>("GET", "/config"),
  staleTime: Infinity,
})

export const connectedAppsQuery = queryOptions({
  queryKey: ["connected-apps"],
  queryFn: () => api<ConnectedApp[]>("GET", "/connected-apps"),
})

// Everything below belongs to one workspace, whose id is the second part of the key.

export const apiKeysQuery = (workspace: string) =>
  queryOptions({
    queryKey: ["api-keys", workspace],
    queryFn: () => api<ApiKey[]>("GET", "/api-keys", { workspace }),
  })

export const oauthAppsQuery = (workspace: string) =>
  queryOptions({
    queryKey: ["oauth-apps", workspace],
    queryFn: () => api<OAuthApp[]>("GET", "/oauth-apps", { workspace }),
  })

export const membersQuery = (workspace: string) =>
  queryOptions({
    queryKey: ["members", workspace],
    queryFn: () => op<Page<Member>>("list_users", { includeInactive: false }, workspace),
    retry: false,
  })

export const invitationsQuery = (workspace: string) =>
  queryOptions({
    queryKey: ["invitations", workspace],
    queryFn: () => op<{ nodes: Invitation[] }>("list_invitations", {}, workspace),
    retry: false,
  })

export const workspaceQuery = (workspace: string) =>
  queryOptions({
    queryKey: ["workspace", workspace],
    queryFn: () => op<Workspace>("get_workspace", {}, workspace),
    retry: false,
  })

export const issueQuery = (workspace: string, id: string) =>
  queryOptions({
    queryKey: ["issue", workspace, id],
    queryFn: () => op<Issue>("get_issue", { id, includeComments: true }, workspace),
    retry: false,
  })

export const projectQuery = (workspace: string, id: string) =>
  queryOptions({
    queryKey: ["project", workspace, id],
    queryFn: () => op<Project>("get_project", { id }, workspace),
    retry: false,
  })

export const documentQuery = (workspace: string, id: string) =>
  queryOptions({
    queryKey: ["document", workspace, id],
    queryFn: () => op<Document>("get_document", { id }, workspace),
    retry: false,
  })

export const initiativeQuery = (workspace: string, id: string) =>
  queryOptions({
    queryKey: ["initiative", workspace, id],
    queryFn: () => op<Initiative>("get_initiative", { id }, workspace),
    retry: false,
  })

export const viewQuery = (workspace: string, id: string) =>
  queryOptions({
    queryKey: ["view", workspace, id],
    queryFn: () => op<CustomView>("get_custom_view", { id }, workspace),
    retry: false,
  })

export const viewIssuesQuery = (workspace: string, id: string) =>
  queryOptions({
    queryKey: ["view-issues", workspace, id],
    queryFn: () => op<Page<IssueSummary>>("list_issues", { customView: id, limit: 100 }, workspace),
    retry: false,
  })

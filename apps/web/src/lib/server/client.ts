import type * as Account from "@/api/account.gen";
import type { OpName, Ops } from "@/api/ops.gen";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

type Method = "GET" | "POST" | "PATCH" | "DELETE";

type Options = {
  body?: unknown;
  /** Workspace URL key or id, sent as x-ricrac-workspace. */
  workspace?: string | null;
};

/** Calls the server. Writes carry the x-ricrac header the server requires against CSRF. */
export async function request<T>(method: Method, url: string, { body, workspace }: Options = {}): Promise<T> {
  const headers: Record<string, string> = { accept: "application/json" };
  if (method !== "GET") headers["x-ricrac"] = "1";
  if (body !== undefined) headers["content-type"] = "application/json";
  if (workspace) headers["x-ricrac-workspace"] = workspace;

  let response: Response;
  try {
    response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: "same-origin",
    });
  } catch {
    throw new ApiError(0, "Couldn't reach Ricrac. Try again in a moment.");
  }
  const data = await response.json().catch(() => undefined);
  if (response.ok) return data as T;
  throw new ApiError(response.status, typeof data?.error === "string" ? data.error : response.statusText);
}

export type OpArgs<N extends OpName> = Ops[N]["args"];
export type OpResult<N extends OpName> = Ops[N]["result"];

/** Runs an operation of the tracker in a workspace, as POST /v1/ops/<name>. */
export function call<N extends OpName>(workspace: string, name: N, args: OpArgs<N>): Promise<OpResult<N>> {
  return request<OpResult<N>>("POST", `/v1/ops/${name}`, { body: args, workspace });
}

/** The dashboard API under /api: the account, workspaces, keys and apps. */
export const account = {
  config: () => request<Account.AppConfig>("GET", "/api/config"),
  me: () => request<Account.Me | null>("GET", "/api/me"),
  updateMe: (body: Account.UpdateMe) => request<Account.Me>("PATCH", "/api/me", { body }),
  signOut: () => request<{ ok: boolean }>("POST", "/api/auth/sign-out"),
  createWorkspace: (body: Account.CreateWorkspace) =>
    request<Account.WorkspaceSummary>("POST", "/api/workspaces", { body }),
  deleteWorkspace: (id: string, body: Account.DeleteWorkspace) =>
    request<unknown>("DELETE", `/api/workspaces/${id}`, { body }),
  leaveWorkspace: (id: string) => request<unknown>("POST", `/api/workspaces/${id}/leave`),
  acceptInvitation: (id: string) => request<Account.WorkspaceSummary>("POST", `/api/invitations/${id}/accept`),
  declineInvitation: (id: string) => request<unknown>("DELETE", `/api/invitations/${id}`),
  apiKeys: () => request<Account.APIKey[]>("GET", "/api/api-keys"),
  createAPIKey: (body: Account.CreateAPIKey) => request<Account.CreatedAPIKey>("POST", "/api/api-keys", { body }),
  deleteAPIKey: (id: string) => request<unknown>("DELETE", `/api/api-keys/${id}`),
  oauthApps: (workspace: string) => request<Account.OAuthApp[]>("GET", "/api/oauth-apps", { workspace }),
  createOAuthApp: (workspace: string, body: Account.SaveOAuthApp) =>
    request<Account.OAuthAppSecret>("POST", "/api/oauth-apps", { body, workspace }),
  updateOAuthApp: (workspace: string, id: string, body: Account.SaveOAuthApp) =>
    request<Account.OAuthApp>("PATCH", `/api/oauth-apps/${id}`, { body, workspace }),
  deleteOAuthApp: (workspace: string, id: string) =>
    request<unknown>("DELETE", `/api/oauth-apps/${id}`, { workspace }),
  rotateOAuthSecret: (workspace: string, id: string) =>
    request<Account.OAuthAppSecret>("POST", `/api/oauth-apps/${id}/secret`, { workspace }),
  connectedApps: () => request<Account.ConnectedApp[]>("GET", "/api/connected-apps"),
  revokeConnectedApp: (id: string) => request<unknown>("DELETE", `/api/connected-apps/${id}`),
  oauthClient: (query: string) => request<Account.OAuthClientInfo>("GET", `/api/oauth/client?${query}`),
  oauthDecide: (body: Account.OAuthDecision) => request<Account.OAuthRedirect>("POST", "/api/oauth/decide", { body }),
};

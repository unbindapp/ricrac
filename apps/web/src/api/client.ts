export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

type Method = "GET" | "POST" | "PATCH" | "DELETE"

type Options = {
  body?: unknown
  /** Workspace id or URL key, sent as x-ricrac-workspace. */
  workspace?: string | null
}

/** Calls the server. Writes carry the x-ricrac header the server requires against CSRF. */
export async function request<T>(method: Method, url: string, { body, workspace }: Options = {}): Promise<T> {
  const headers: Record<string, string> = { accept: "application/json" }
  if (method !== "GET") headers["x-ricrac"] = "1"
  if (body !== undefined) headers["content-type"] = "application/json"
  if (workspace) headers["x-ricrac-workspace"] = workspace

  let response: Response
  try {
    response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: "same-origin",
    })
  } catch {
    throw new ApiError(0, "Couldn't reach Ricrac. Try again in a moment.")
  }
  const data = await response.json().catch(() => undefined)
  if (response.ok) return data as T
  throw new ApiError(response.status, typeof data?.error === "string" ? data.error : response.statusText)
}

/** The dashboard API under /api. */
export const api = <T>(method: Method, path: string, options?: Options) => request<T>(method, `/api${path}`, options)

/** An operation of the tracker, through POST /v1/ops/<name> with the session. */
export const op = <T>(name: string, args: object, workspace: string) =>
  request<T>("POST", `/v1/ops/${name}`, { body: args, workspace })

export const errorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error))

/** The server doesn't have this operation yet. */
export const isMissingOp = (error: unknown) =>
  error instanceof ApiError && error.status === 404 && error.message.startsWith("No operation")

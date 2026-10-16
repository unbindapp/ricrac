import { useMutation, useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { CheckIcon, ShieldAlertIcon, TriangleAlertIcon } from "lucide-react"
import { useState } from "react"

import { api, errorMessage } from "@/api/client"
import { meQuery } from "@/api/queries"
import type { Me } from "@/api/types/Me"
import type { OAuthClientInfo } from "@/api/types/OAuthClientInfo"
import type { OAuthDecision } from "@/api/types/OAuthDecision"
import type { OAuthRedirect } from "@/api/types/OAuthRedirect"
import type { OAuthRequest } from "@/api/types/OAuthRequest"
import { CenterPanel } from "@/components/common"
import { CreateWorkspaceForm } from "@/components/create-workspace"
import { SignedInAs } from "@/components/shell"
import { SignedOut } from "@/components/sign-in"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldLabel } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { useSelectedWorkspace } from "@/lib/workspace"

/** The OAuth consent screen that MCP clients and OAuth apps send people to. */
export const Route = createFileRoute("/oauth/authorize")({ component: AuthorizePage })

const scopeWords: Record<string, string> = {
  read: "Read everything you can see in the workspace",
  write: "Create and change issues, projects, comments and more",
  admin: "Workspace administration: members, teams and settings",
}

function oauthRequest(search: string): OAuthRequest {
  const query = new URLSearchParams(search)
  const optional = (key: string) => query.get(key) ?? undefined
  return {
    client_id: query.get("client_id") ?? "",
    redirect_uri: query.get("redirect_uri") ?? "",
    response_type: query.get("response_type") ?? "",
    code_challenge: optional("code_challenge"),
    code_challenge_method: optional("code_challenge_method"),
    state: optional("state"),
    scope: optional("scope"),
    resource: optional("resource"),
  }
}

function AuthorizePage() {
  const { data: me } = useQuery(meQuery)
  const search = window.location.search
  const info = useQuery({
    queryKey: ["oauth-client", search],
    queryFn: () => api<OAuthClientInfo>("GET", `/oauth/client${search}`),
    retry: false,
    enabled: !!me,
  })

  if (!me) {
    return <SignedOut title="Sign in to connect" description="An app wants to connect to your Ricrac workspace." />
  }
  if (info.isPending) {
    return (
      <div className="grid min-h-svh place-items-center text-muted-foreground" role="status" aria-label="Loading">
        <Spinner className="size-5" />
      </div>
    )
  }
  if (info.isError) {
    return (
      <CenterPanel>
        <Alert variant="destructive">
          <TriangleAlertIcon />
          <AlertTitle>Can't connect this app</AlertTitle>
          <AlertDescription>{errorMessage(info.error)}</AlertDescription>
        </Alert>
      </CenterPanel>
    )
  }
  return <Consent me={me} info={info.data} request={oauthRequest(search)} />
}

function Consent({ me, info, request }: { me: Me; info: OAuthClientInfo; request: OAuthRequest }) {
  const selected = useSelectedWorkspace(me)
  const [picked, setWorkspaceId] = useState<string | null>(null)
  const workspaceId = picked ?? selected?.id ?? ""
  const [creating, setCreating] = useState(false)
  const decide = useMutation({
    mutationFn: (allow: boolean) =>
      api<OAuthRedirect>("POST", "/oauth/decide", {
        body: { request, workspaceId: workspaceId || undefined, allow } satisfies OAuthDecision,
      }),
    // Stays busy while the browser goes on to the app.
    onSuccess: ({ redirect }) => {
      window.location.href = redirect
    },
  })
  const busy = decide.isPending || decide.isSuccess

  if (me.workspaces.length === 0 || creating) {
    return (
      <CenterPanel wide>
        <Card>
          <CardHeader>
            <CardTitle>Create a workspace first</CardTitle>
            <CardDescription>
              {info.client_name} connects to one workspace. Create yours, then come back to allow it.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <CreateWorkspaceForm
              onCreated={() => setCreating(false)}
              onCancel={me.workspaces.length > 0 ? () => setCreating(false) : undefined}
            />
          </CardContent>
        </Card>
        <SignedInAs me={me} />
      </CenterPanel>
    )
  }

  return (
    <CenterPanel wide>
      <Card>
        <CardHeader>
          <CardTitle className="text-xl leading-snug break-words">
            {info.client_name} wants to access your Ricrac workspace
          </CardTitle>
          <CardDescription>
            After you decide, you'll go back to{" "}
            <span className="font-mono break-all text-foreground">{info.redirect_host}</span>.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {!info.verified && (
            <Alert>
              <ShieldAlertIcon />
              <AlertTitle>Not verified</AlertTitle>
              <AlertDescription>
                This app registered itself, so Ricrac can't vouch for its name. Only allow it if you started connecting
                it just now.
              </AlertDescription>
            </Alert>
          )}
          <div className="space-y-2">
            <p className="text-sm font-medium">It will be able to</p>
            <ul className="space-y-2">
              {info.scopes.map((scope) => (
                <li key={scope} className="flex gap-2 text-sm">
                  <CheckIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
                  <span>{scopeWords[scope] ?? scope}</span>
                </li>
              ))}
            </ul>
            <p className="text-xs text-muted-foreground">It acts as you, with your permissions.</p>
          </div>
          <Field>
            <FieldLabel htmlFor="consent-workspace">Workspace</FieldLabel>
            <Select
              value={workspaceId}
              onValueChange={(value) => (value === "__new" ? setCreating(true) : setWorkspaceId(value))}
            >
              <SelectTrigger id="consent-workspace" className="w-full">
                <SelectValue placeholder="Pick a workspace" />
              </SelectTrigger>
              <SelectContent>
                {me.workspaces.map((workspace) => (
                  <SelectItem key={workspace.id} value={workspace.id}>
                    {workspace.name}
                  </SelectItem>
                ))}
                <SelectItem value="__new">Create a workspace…</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          {decide.isError && (
            <Alert variant="destructive">
              <TriangleAlertIcon />
              <AlertDescription>{errorMessage(decide.error)}</AlertDescription>
            </Alert>
          )}
        </CardContent>
        <CardFooter className="grid grid-cols-2 gap-3">
          <Button variant="outline" disabled={busy} onClick={() => decide.mutate(false)}>
            Deny
          </Button>
          <Button disabled={busy || !workspaceId} onClick={() => decide.mutate(true)}>
            {busy && <Spinner />}
            Allow
          </Button>
        </CardFooter>
      </Card>
      <SignedInAs me={me} />
    </CenterPanel>
  )
}

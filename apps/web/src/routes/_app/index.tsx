import { useQuery } from "@tanstack/react-query"
import { Link, createFileRoute } from "@tanstack/react-router"
import { ExternalLinkIcon } from "lucide-react"
import type { ReactNode } from "react"

import { configQuery } from "@/api/queries"
import { CodeBlock, CopyValue, PageHeader } from "@/components/common"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

export const Route = createFileRoute("/_app/")({ component: ConnectPage })

function ConnectPage() {
  const { data: config } = useQuery(configQuery)
  if (!config) return null
  const mcpUrl = config.mcpUrl
  const apiBase = `${config.publicUrl}/v1`
  const clients: { id: string; name: string; steps: ReactNode }[] = [
    {
      id: "claude",
      name: "Claude",
      steps: (
        <Steps>
          <li>In claude.ai or Claude Desktop, open Settings → Connectors.</li>
          <li>Choose “Add custom connector”, name it Ricrac and paste the MCP URL.</li>
          <li>Connect, sign in with Google and allow it (all your workspaces, or the ones you pick).</li>
        </Steps>
      ),
    },
    {
      id: "claude-code",
      name: "Claude Code",
      steps: (
        <>
          <p>Run this, then use /mcp in Claude Code to sign in:</p>
          <CodeBlock code={`claude mcp add --transport http ricrac ${mcpUrl}`} />
        </>
      ),
    },
    {
      id: "chatgpt",
      name: "ChatGPT",
      steps: (
        <Steps>
          <li>Open Settings → Apps & Connectors → Advanced and turn on Developer mode.</li>
          <li>Create a connector with the MCP URL and OAuth authentication.</li>
          <li>Sign in with Google when asked and allow it.</li>
        </Steps>
      ),
    },
    {
      id: "codex",
      name: "Codex",
      steps: (
        <>
          <p>Run this; Codex opens the browser to sign in:</p>
          <CodeBlock code={`codex mcp add ricrac --url ${mcpUrl}`} />
        </>
      ),
    },
    {
      id: "cursor",
      name: "Cursor",
      steps: (
        <>
          <p>Add Ricrac to ~/.cursor/mcp.json (or .cursor/mcp.json in a project):</p>
          <CodeBlock code={JSON.stringify({ mcpServers: { ricrac: { url: mcpUrl } } }, null, 2)} />
        </>
      ),
    },
  ]

  return (
    <>
      <PageHeader title="Connect" description="Agents use Ricrac through its MCP server or the REST API." />
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>MCP server</CardTitle>
            <CardDescription>
              Add this URL to your agent. It signs in with OAuth and asks which workspaces it may use: all of them,
              including ones you join later, or only the ones you pick. Everyday operations are listed as tools and the rest are reached through
              find_operations; add <code className="font-mono">?tools=all</code> to the URL to list every operation.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <CopyValue value={mcpUrl} label="MCP URL" />
            <Tabs defaultValue="claude">
              <TabsList className="flex-wrap">
                {clients.map((client) => (
                  <TabsTrigger key={client.id} value={client.id}>
                    {client.name}
                  </TabsTrigger>
                ))}
              </TabsList>
              {clients.map((client) => (
                <TabsContent key={client.id} value={client.id} className="space-y-3 pt-3 text-sm">
                  {client.steps}
                </TabsContent>
              ))}
            </Tabs>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>REST API</CardTitle>
            <CardDescription>
              Every MCP tool is also an HTTP endpoint. Authenticate with an{" "}
              <Link to="/settings/api-keys" className="underline underline-offset-4 hover:text-foreground">
                API key
              </Link>{" "}
              or an OAuth access token.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <p className="text-sm font-medium">Base URL</p>
              <CopyValue value={apiBase} label="API base URL" />
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
              <DocLink href="/v1/openapi.json">OpenAPI document</DocLink>
              <DocLink href="/llms.txt">Guide for agents (llms.txt)</DocLink>
            </div>
            <div className="space-y-1.5">
              <p className="text-sm font-medium">Example</p>
              <CodeBlock
                code={`curl ${apiBase}/ops/list_issues \\\n  -H "Authorization: Bearer <API key>" \\\n  -H "content-type: application/json" \\\n  -d '{"assignee": "me", "limit": 10}'`}
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </>
  )
}

function Steps({ children }: { children: ReactNode }) {
  return <ol className="list-decimal space-y-1.5 pl-5 marker:text-muted-foreground">{children}</ol>
}

function DocLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-1.5 underline-offset-4 hover:underline"
    >
      {children}
      <ExternalLinkIcon className="size-3.5 text-muted-foreground" aria-hidden />
    </a>
  )
}

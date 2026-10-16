import { useQuery } from "@tanstack/react-query"
import { useLocation } from "@tanstack/react-router"
import { InfoIcon, TriangleAlertIcon } from "lucide-react"
import { useState } from "react"

import { configQuery } from "@/api/queries"
import { CenterPanel } from "@/components/common"
import { GoogleIcon } from "@/components/google-icon"
import { PublicFooter } from "@/components/public-page"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"

const signInErrors: Record<string, string> = {
  expired: "The sign-in took too long or was interrupted. Please try again.",
  denied: "Sign-in was canceled in Google.",
  failed: "Signing in with Google didn't work. Please try again.",
  unverified: "Your Google account's email address isn't verified yet.",
  not_configured: "Sign-in with Google isn't set up on this server yet.",
}

/** The sign-in link, coming back to the current page (without a previous error). */
function useSignInHref() {
  const location = useLocation()
  const search = new URLSearchParams(location.searchStr)
  const error = search.get("signin_error")
  search.delete("signin_error")
  const returnTo = location.pathname + (search.size ? `?${search}` : "")
  return { href: `/api/auth/sign-in?return_to=${encodeURIComponent(returnTo)}`, error }
}

export function GoogleButton() {
  const { href } = useSignInHref()
  const [leaving, setLeaving] = useState(false)
  return (
    <Button asChild size="lg" variant="outline" className="w-full">
      <a href={href} onClick={() => setLeaving(true)}>
        {leaving ? <Spinner /> : <GoogleIcon />}
        Continue with Google
      </a>
    </Button>
  )
}

export function SignInError() {
  const { error } = useSignInHref()
  if (!error) return null
  return (
    <Alert variant="destructive">
      <TriangleAlertIcon />
      <AlertTitle>Couldn't sign you in</AlertTitle>
      <AlertDescription>{signInErrors[error] ?? signInErrors.failed}</AlertDescription>
    </Alert>
  )
}

export function GoogleNotConfigured() {
  const { data: config } = useQuery(configQuery)
  if (!config || config.googleConfigured) return null
  return (
    <Alert>
      <InfoIcon />
      <AlertTitle>Google sign-in isn't configured</AlertTitle>
      <AlertDescription>
        Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET on the server to let people sign in. API keys and OAuth tokens
        keep working without it.
      </AlertDescription>
    </Alert>
  )
}

/** The page visitors see. */
export function SignedOut({ title = "Sign in", description }: { title?: string; description?: string }) {
  return (
    <CenterPanel>
      <div className="space-y-2 text-center">
        <p className="text-balance text-muted-foreground">
          Issue tracking for agents, over MCP and a REST API.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>
            {description ?? "Manage your workspaces, members, API keys and the apps connected to Ricrac."}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <SignInError />
          <GoogleNotConfigured />
          <GoogleButton />
        </CardContent>
      </Card>
      <PublicFooter />
    </CenterPanel>
  )
}

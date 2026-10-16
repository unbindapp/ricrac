import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { api } from "@/api/client"
import { meQuery } from "@/api/queries"
import type { Me } from "@/api/types/Me"
import type { UpdateMe } from "@/api/types/UpdateMe"
import { PageHeader } from "@/components/common"
import { UserAvatar } from "@/components/shell"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardFooter } from "@/components/ui/card"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"

export const Route = createFileRoute("/_app/settings/profile")({ component: ProfilePage })

function ProfilePage() {
  const { data: me } = useQuery(meQuery)
  if (!me) return null
  return (
    <>
      <PageHeader title="Profile" description="How you appear to people and agents in your workspaces." />
      <ProfileForm me={me} />
    </>
  )
}

function ProfileForm({ me }: { me: Me }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(me.name)
  const [displayName, setDisplayName] = useState(me.displayName)
  const changed = name.trim() !== me.name || displayName.trim() !== me.displayName

  const save = useMutation({
    mutationFn: (body: UpdateMe) => api("PATCH", "/me", { body }),
    onSuccess: async () => {
      toast.success("Profile saved")
      await queryClient.invalidateQueries({ queryKey: meQuery.queryKey })
    },
    onError: (error) => toast.error(error.message),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    save.mutate({
      name: name.trim() !== me.name ? name.trim() : undefined,
      displayName: displayName.trim() !== me.displayName ? displayName.trim() : undefined,
    })
  }

  return (
    <Card className="max-w-2xl">
      <form onSubmit={submit} className="contents">
        <CardContent>
          <div className="mb-6 flex items-center gap-4">
            <UserAvatar me={me} className="size-12" />
            <div className="min-w-0">
              <div className="truncate font-medium">{me.name}</div>
              <div className="truncate text-sm text-muted-foreground">{me.email}</div>
            </div>
          </div>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="profile-name">Name</FieldLabel>
              <Input
                id="profile-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                maxLength={100}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="profile-handle">Display name</FieldLabel>
              <div className="relative">
                <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-sm text-muted-foreground">
                  @
                </span>
                <Input
                  id="profile-handle"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  required
                  maxLength={40}
                  pattern="[\p{L}\p{N}._\-]+"
                  className="pl-7"
                  aria-describedby="profile-handle-help"
                />
              </div>
              <FieldDescription id="profile-handle-help">
                Used for @mentions. Letters, digits, dots, dashes and underscores.
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="profile-email">Email</FieldLabel>
              <Input id="profile-email" value={me.email} readOnly disabled aria-describedby="profile-email-help" />
              <FieldDescription id="profile-email-help">From your Google account.</FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter className="justify-end">
          <Button type="submit" disabled={!changed || save.isPending}>
            {save.isPending && <Spinner />}
            Save
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}

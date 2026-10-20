import type { AnyFieldApi } from "@tanstack/react-form";
import { z } from "zod";

import type { Invitation } from "@/api/ops.gen";
import { useInviteStore } from "@/components/settings/invite-store";
import { CheckboxList, type TCheckboxListItem } from "@/components/shared/checkbox-list";
import { roleItems } from "@/components/shared/role-select";
import { TeamIcon } from "@/components/shared/team-icon";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "@/components/ui/toast";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm, useFieldError } from "@/lib/hooks/use-app-form";
import {
  INVITABLE_ROLES,
  useInviteUserMutation,
  workspaceRoleDescriptions,
} from "@/lib/queries/workspaces";

type TInvitableRole = Invitation["role"];

const formSchema = z
  .object({
    email: z.string().trim().email("Enter a valid email address."),
    role: z.enum(["admin", "member", "guest"]),
    teams: z.array(z.string()),
  })
  .refine((value) => value.role !== "guest" || value.teams.length > 0, {
    message: "Guests only see their teams. Pick at least one.",
    path: ["teams"],
  });

export function InviteMemberDialog() {
  const isOpen = useInviteStore((state) => state.isOpen);
  const setIsOpen = useInviteStore((state) => state.setOpen);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>Invite people</DialogTitle>
          <DialogDescription>
            They join by signing in as this email, with Google or a link sent to it. Ricrac emails
            them the invitation when email is set up.
          </DialogDescription>
        </DialogHeader>
        {isOpen && <InviteForm onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function InviteForm({ onDone }: { onDone: () => void }) {
  const { workspace, isAdmin } = useWorkspace();
  const { teams } = useTeams();
  const { mutateAsync: invite } = useInviteUserMutation(workspace.id);
  const roles = INVITABLE_ROLES.filter((role) => isAdmin || role !== "admin");
  const form = useAppForm({
    defaultValues: { email: "", role: "member" as TInvitableRole, teams: [] as string[] },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      const invitation = await invite(parsed);
      toast.add({ type: "success", title: `Invited ${invitation.email}` });
      onDone();
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="email"
        children={(field) => (
          <field.TextField label="Email" type="email" placeholder="name@example.com" autoFocus />
        )}
      />
      <form.Subscribe selector={(state) => state.values.role}>
        {(role) => (
          <form.AppField
            name="role"
            children={(field) => (
              <field.SelectField
                label="Role"
                description={workspaceRoleDescriptions[role]}
                items={roleItems(roles)}
              />
            )}
          />
        )}
      </form.Subscribe>
      <form.Field
        name="teams"
        children={(field) => (
          <TeamsField
            field={field}
            items={teams.map((team) => ({
              value: team.key,
              label: team.name,
              description: team.key,
              icon: <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />,
            }))}
          />
        )}
      />
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">Send invitation</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

function TeamsField({ field, items }: { field: AnyFieldApi; items: TCheckboxListItem[] }) {
  const { error } = useFieldError(field);
  return (
    <div className="flex flex-col gap-1.5">
      <p className="text-sm font-medium">Teams</p>
      <p className="text-muted-foreground text-xs">The teams they join. Required for guests.</p>
      <CheckboxList
        label="Teams"
        emptyText="There are no teams yet."
        items={items}
        value={field.state.value as string[]}
        onChange={(value) => {
          field.handleChange(value);
          field.handleBlur();
        }}
      />
      {error && (
        <p role="alert" className="text-destructive text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

import { z } from "zod";

import type { Invitation } from "@/api/ops.gen";
import { useInviteStore } from "@/components/settings/invite-store";
import { roleItems } from "@/components/shared/role-select";
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
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import {
  INVITABLE_ROLES,
  useInviteUserMutation,
  workspaceRoleDescriptions,
} from "@/lib/queries/workspaces";

type TInvitableRole = Invitation["role"];

const formSchema = z.object({
  email: z.string().trim().email("Enter a valid email address."),
  role: z.enum(["admin", "member", "guest"]),
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
  const team = useWorkspaceTeam();
  const { mutateAsync: invite } = useInviteUserMutation(workspace.id);
  const roles = INVITABLE_ROLES.filter((role) => isAdmin || role !== "admin");
  const form = useAppForm({
    defaultValues: { email: "", role: "member" as TInvitableRole },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      const invitation = await invite({ ...parsed, teams: team ? [team.id] : [] });
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

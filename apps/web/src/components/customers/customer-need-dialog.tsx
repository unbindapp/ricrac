import { useStore } from "@tanstack/react-form";
import { type ReactElement, useId, useState } from "react";
import { z } from "zod";

import type { CustomerNeed } from "@/api/ops.gen";
import {
  CustomerTargetPicker,
  IssueTargetPicker,
  ProjectTargetPicker,
  type TPicked,
} from "@/components/customers/need-target-pickers";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { type TNeedScope, useSaveCustomerNeedMutation } from "@/lib/queries/customers";

type TTargetType = "issue" | "project";

const picked = z.object({ id: z.string(), label: z.string() }).nullable();

const formSchema = z.object({
  customer: picked,
  targetType: z.enum(["issue", "project"]),
  target: picked,
  important: z.boolean(),
  body: z.string().max(10000, "Keep it under 10,000 characters."),
  attachmentUrl: z
    .string()
    .trim()
    .refine((value) => value === "" || /^https?:\/\/\S+$/.test(value), {
      message: "Use a link starting with https://.",
    }),
});

type TProps = {
  trigger: ReactElement;
  /** Who or what the request is about: the dialog asks for the rest. */
  scope: TNeedScope;
  need?: CustomerNeed;
};

export function CustomerNeedDialog({ trigger, scope, need }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{need ? "Edit request" : "Add customer request"}</DialogTitle>
          <DialogDescription>
            What a customer asked for, linked to the issue or project that delivers it.
          </DialogDescription>
        </DialogHeader>
        {isOpen && <NeedForm scope={scope} need={need} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function NeedForm({
  scope,
  need,
  onDone,
}: {
  scope: TNeedScope;
  need?: CustomerNeed;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const ids = { customer: useId(), target: useId() };
  const { mutateAsync: save } = useSaveCustomerNeedMutation(workspace.id);
  const asksCustomer = !need && !("customer" in scope);
  const asksTarget = !need && "customer" in scope;
  const form = useAppForm({
    defaultValues: {
      customer: null as TPicked | null,
      targetType: "issue" as TTargetType,
      target: null as TPicked | null,
      important: (need?.priority ?? 0) === 1,
      body: need?.body ?? "",
      attachmentUrl: need?.attachmentUrl ?? "",
    },
    validators: {
      onSubmit: formSchema
        .refine((value) => !asksCustomer || value.customer, {
          message: "Pick a customer.",
          path: ["customer"],
        })
        .refine((value) => !asksTarget || value.target, {
          message: "Pick what they need.",
          path: ["target"],
        }),
    },
    onSubmit: async ({ value }) => {
      const fields = {
        priority: value.important ? 1 : 0,
        body: value.body.trim() || null,
        attachmentUrl: value.attachmentUrl.trim() || null,
      };
      if (need) {
        await save({ id: need.id, ...fields });
        onDone();
        return;
      }
      const target = asksTarget ? { [value.targetType]: value.target?.id } : scope;
      const customer = asksCustomer ? { customer: value.customer?.id } : {};
      await save({ ...target, ...customer, ...fields });
      onDone();
    },
  });
  const targetType = useStore(form.store, (state) => state.values.targetType);
  const submitted = useStore(form.store, (state) => state.submissionAttempts > 0);

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      {asksCustomer && (
        <form.Field
          name="customer"
          children={(field) => (
            <div className="flex flex-col gap-1.5">
              <label htmlFor={ids.customer} className="text-sm font-medium">
                Customer
              </label>
              <CustomerTargetPicker
                id={ids.customer}
                value={field.state.value}
                invalid={submitted && !field.state.value}
                onChange={field.handleChange}
              />
              <FieldError show={submitted && !field.state.value} text="Pick a customer." />
            </div>
          )}
        />
      )}
      {asksTarget && (
        <form.Field
          name="target"
          children={(field) => (
            <div className="flex flex-col gap-1.5">
              <div className="flex items-center gap-2">
                <label htmlFor={ids.target} className="flex-1 text-sm font-medium">
                  They need
                </label>
                <div role="radiogroup" aria-label="Kind" className="flex items-center gap-1">
                  {(["issue", "project"] as const).map((type) => (
                    <Button
                      key={type}
                      type="button"
                      role="radio"
                      aria-checked={targetType === type}
                      variant={targetType === type ? "secondary" : "ghost"}
                      size="sm"
                      onClick={() => {
                        form.setFieldValue("targetType", type);
                        field.handleChange(null);
                      }}
                    >
                      {type === "issue" ? "Issue" : "Project"}
                    </Button>
                  ))}
                </div>
              </div>
              {targetType === "issue" ? (
                <IssueTargetPicker
                  id={ids.target}
                  value={field.state.value}
                  invalid={submitted && !field.state.value}
                  onChange={field.handleChange}
                />
              ) : (
                <ProjectTargetPicker
                  id={ids.target}
                  value={field.state.value}
                  invalid={submitted && !field.state.value}
                  onChange={field.handleChange}
                />
              )}
              <FieldError show={submitted && !field.state.value} text="Pick what they need." />
            </div>
          )}
        />
      )}
      <form.AppField
        name="body"
        children={(field) => (
          <field.TextareaField
            label="What they said"
            placeholder="Quote the customer, or sum up their request…"
            rows={4}
          />
        )}
      />
      <form.AppField
        name="attachmentUrl"
        children={(field) => (
          <field.TextField
            label="Source"
            placeholder="https://"
            description="Optional: a link to the email, call notes or ticket."
            inputMode="url"
          />
        )}
      />
      <form.AppField
        name="important"
        children={(field) => (
          <field.SwitchField
            label="Important"
            description="The customer said this matters a lot to them."
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
          <form.SubmitButton size="sm">{need ? "Save" : "Add request"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

function FieldError({ show, text }: { show: boolean; text: string }) {
  if (!show) return null;
  return (
    <p role="alert" className="text-destructive text-xs">
      {text}
    </p>
  );
}

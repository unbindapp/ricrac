import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { Customer } from "@/api/ops.gen";
import { OptionDot } from "@/components/customers/customer-badges";
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
import { parseDomains, parseWholeNumber } from "@/lib/helpers/customers";
import { submitHandler, type TSelectFieldItem, useAppForm } from "@/lib/hooks/use-app-form";
import {
  CustomerFormSchema,
  customerStatusesQuery,
  customerTiersQuery,
  useSaveCustomerMutation,
} from "@/lib/queries/customers";

const NONE = "none";

const formSchema = CustomerFormSchema.extend({
  status: z.string(),
  tier: z.string(),
  logoUrl: z
    .string()
    .trim()
    .refine((value) => value === "" || /^https?:\/\/\S+$/.test(value), {
      message: "Use a link to an image, starting with https://.",
    }),
  externalIds: z.string(),
});

type TProps = {
  trigger: ReactElement;
  /** Edits this customer's details; without it, the dialog creates one. */
  customer?: Customer;
};

export function CustomerDialog({ trigger, customer }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{customer ? "Edit customer" : "New customer"}</DialogTitle>
          <DialogDescription>
            {customer
              ? "Their status, tier and owner are in the properties panel."
              : "Link their requests to issues and projects to see who needs what."}
          </DialogDescription>
        </DialogHeader>
        {isOpen && <CustomerForm customer={customer} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function optionItems(options: { id: string; name: string; color: string }[], none: string) {
  return [
    { value: NONE, label: none },
    ...options.map((option): TSelectFieldItem => ({
      value: option.id,
      label: option.name,
      icon: <OptionDot color={option.color} />,
    })),
  ];
}

function CustomerForm({ customer, onDone }: { customer?: Customer; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { data: statuses = [] } = useQuery(customerStatusesQuery(workspace.id));
  const { data: tiers = [] } = useQuery(customerTiersQuery(workspace.id));
  const { mutateAsync: save } = useSaveCustomerMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      name: customer?.name ?? "",
      domains: customer?.domains.join(", ") ?? "",
      status: statuses[0]?.id ?? NONE,
      tier: NONE,
      revenue: customer?.revenue?.toString() ?? "",
      size: customer?.size?.toString() ?? "",
      logoUrl: customer?.logoUrl ?? "",
      externalIds: customer?.externalIds.join(", ") ?? "",
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const fields = {
        name: value.name.trim(),
        domains: parseDomains(value.domains),
        revenue: parseWholeNumber(value.revenue) ?? null,
        size: parseWholeNumber(value.size) ?? null,
      };
      if (customer) {
        await save({
          id: customer.id,
          ...fields,
          logoUrl: value.logoUrl.trim() || null,
          externalIds: value.externalIds
            .split(",")
            .map((id) => id.trim())
            .filter(Boolean),
        });
        onDone();
        return;
      }
      const created = await save({
        ...fields,
        status: value.status === NONE ? null : value.status,
        tier: value.tier === NONE ? null : value.tier,
      });
      onDone();
      void navigate({
        to: "/$workspace/customer/$slug",
        params: { workspace: workspace.urlKey, slug: created.slugId },
      });
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        children={(field) => (
          <field.TextField label="Name" placeholder="Acme Inc." autoComplete="off" autoFocus />
        )}
      />
      <form.AppField
        name="domains"
        children={(field) => (
          <field.TextField
            label="Domains"
            placeholder="acme.com"
            description="Separate several with commas. Each domain belongs to one customer."
            autoComplete="off"
          />
        )}
      />
      <div className="grid grid-cols-2 gap-4">
        {!customer && (
          <>
            <form.AppField
              name="status"
              children={(field) => (
                <field.SelectField label="Status" items={optionItems(statuses, "No status")} />
              )}
            />
            <form.AppField
              name="tier"
              children={(field) => (
                <field.SelectField label="Tier" items={optionItems(tiers, "No tier")} />
              )}
            />
          </>
        )}
        <form.AppField
          name="revenue"
          children={(field) => (
            <field.TextField label="Annual revenue" placeholder="120,000" inputMode="numeric" />
          )}
        />
        <form.AppField
          name="size"
          children={(field) => (
            <field.TextField label="Employees" placeholder="50" inputMode="numeric" />
          )}
        />
      </div>
      {customer && (
        <>
          <form.AppField
            name="logoUrl"
            children={(field) => (
              <field.TextField
                label="Logo"
                placeholder="https://acme.com/logo.png"
                inputMode="url"
              />
            )}
          />
          <form.AppField
            name="externalIds"
            children={(field) => (
              <field.TextField
                label="External ids"
                placeholder="cus_123, hubspot-42"
                description="Their ids in your CRM or billing, separated by commas."
                autoComplete="off"
              />
            )}
          />
        </>
      )}
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">{customer ? "Save" : "Create customer"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

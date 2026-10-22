import { createFileRoute } from "@tanstack/react-router";

import { CustomerOptionSection } from "@/components/customers/customer-option-settings";
import { SettingsPage } from "@/components/layout/settings-page";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/_admin/customers",
)({
  component: CustomerSettingsPage,
});

function CustomerSettingsPage() {
  return (
    <SettingsPage title="Customers" description="The statuses and tiers that sort your customers.">
      <CustomerOptionSection kind="status" />
      <CustomerOptionSection kind="tier" />
    </SettingsPage>
  );
}

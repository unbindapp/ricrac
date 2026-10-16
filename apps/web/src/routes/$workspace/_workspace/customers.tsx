import { createFileRoute } from "@tanstack/react-router";

import { CustomersView } from "@/components/customers/customers-view";

export const Route = createFileRoute("/$workspace/_workspace/customers")({
  component: CustomersView,
});

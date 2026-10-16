import { createFileRoute } from "@tanstack/react-router";

import { CustomerDetail } from "@/components/customers/customer-detail";

export const Route = createFileRoute("/$workspace/_workspace/customer/$slug")({
  component: CustomerPage,
});

function CustomerPage() {
  const { slug } = Route.useParams();
  return <CustomerDetail key={slug} slug={slug} />;
}

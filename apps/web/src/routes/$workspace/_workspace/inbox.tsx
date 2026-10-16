import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { INBOX_FILTERS, InboxView } from "@/components/inbox/inbox-view";

const searchSchema = z.object({
  filter: z.enum(INBOX_FILTERS).optional(),
  /** The open row: issue:<id>, project:<id>, ... */
  open: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/inbox")({
  validateSearch: zodValidator(searchSchema),
  component: InboxPage,
});

function InboxPage() {
  const { filter, open } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <InboxView
      filter={filter ?? "inbox"}
      selectedKey={open}
      onFilterChange={(next) =>
        void navigate({ search: { filter: next === "inbox" ? undefined : next }, replace: true })
      }
      onSelect={(next) =>
        void navigate({ search: (prev) => ({ ...prev, open: next }), replace: true })
      }
    />
  );
}

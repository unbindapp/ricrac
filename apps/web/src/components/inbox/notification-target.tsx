import { IssueDetail } from "@/components/issues/issue-detail";

/** The issues area's detail, as the inbox shows it beside the list. */
export function NotificationIssue({ identifier }: { identifier: string }) {
  return <IssueDetail key={identifier} identifier={identifier} variant="peek" shortcuts={false} />;
}

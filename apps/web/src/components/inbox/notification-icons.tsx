import {
  AtSignIcon,
  BellIcon,
  BoxIcon,
  CalendarClockIcon,
  CheckCheckIcon,
  CircleArrowRightIcon,
  CircleCheckIcon,
  CircleXIcon,
  FileTextIcon,
  FlagIcon,
  type LucideIcon,
  MegaphoneIcon,
  MessageSquareIcon,
  MessagesSquareIcon,
  PencilLineIcon,
  ReplyIcon,
  Share2Icon,
  SplitIcon,
  TargetIcon,
  TriangleIcon,
  UnlockIcon,
  UserMinusIcon,
  UserPlusIcon,
} from "lucide-react";

import type { TNotificationTarget } from "@/components/inbox/notifications";
import { StateIcon } from "@/components/shared/state-icon";

const typeIcons: Record<string, LucideIcon> = {
  issueAssignedToYou: UserPlusIcon,
  issueUnassignedFromYou: UserMinusIcon,
  issueStatusChanged: CircleArrowRightIcon,
  issueUnblocked: UnlockIcon,
  issueDue: CalendarClockIcon,
  issueShared: Share2Icon,
  triageIssue: TriangleIcon,
  projectAssigned: FlagIcon,
  projectUpdateCreated: MegaphoneIcon,
  initiativeUpdateCreated: MegaphoneIcon,
  documentSuggestion: PencilLineIcon,
  documentSuggestionAccepted: CheckCheckIcon,
  documentSuggestionRejected: CircleXIcon,
  discussionNewThread: SplitIcon,
  discussionThreadResolved: CircleCheckIcon,
};

/** The icon of what happened: a mention, a comment, a reply, an assignment... */
export function notificationTypeIcon(type: string): LucideIcon {
  if (typeIcons[type]) return typeIcons[type];
  if (type.endsWith("Mention")) return AtSignIcon;
  if (type.endsWith("Reply")) return ReplyIcon;
  if (type.endsWith("Comment")) return MessageSquareIcon;
  return BellIcon;
}

const targetIcons: Record<Exclude<TNotificationTarget["kind"], "issue">, LucideIcon> = {
  project: BoxIcon,
  initiative: TargetIcon,
  document: FileTextIcon,
  discussion: MessagesSquareIcon,
  none: BellIcon,
};

export function NotificationTargetIcon({ target }: { target: TNotificationTarget }) {
  if (target.kind === "issue") {
    return (
      <StateIcon type={target.state.type} color={target.state.color} name={target.state.name} />
    );
  }
  const Icon = targetIcons[target.kind];
  return <Icon className="text-muted-foreground size-4 shrink-0" />;
}

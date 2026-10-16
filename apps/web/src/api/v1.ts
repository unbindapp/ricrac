// The /v1 shapes the dashboard reads. Fields the server may not send yet are optional.

export type Page<T> = { nodes: T[]; pageInfo: { hasNextPage: boolean; endCursor: string | null } }

export type UserRef = { id: string; name: string; displayName: string; email: string }
export type StateRef = { id: string; name: string; type: string; color: string }
export type TeamRef = { id: string; key: string; name: string }
export type LabelRef = { id: string; name: string; color: string; parent: string | null }
export type IssueRef = { id: string; identifier: string; title: string; state: StateRef | null }
export type SlugRef = { id: string; name: string; slugId: string }
export type MilestoneRef = { id: string; name: string; targetDate: string | null }
export type CycleRef = { id: string; number: number; name: string | null; startsAt: string; endsAt: string }

export type Role = "owner" | "admin" | "member" | "guest"

export type Member = {
  id: string
  name: string
  displayName: string
  email: string
  avatarUrl: string | null
  role: Role
  active: boolean
  isMe: boolean
  joinedAt: string
  teams: TeamRef[]
}

export type Invitation = {
  id: string
  email: string
  role: Role
  teams: TeamRef[]
  inviter: UserRef | null
  createdAt: string
  expiresAt: string
}

export type Workspace = {
  id: string
  name: string
  urlKey: string
  logoUrl: string | null
  url: string
  role: Role
  teams: { id: string; key: string; name: string; private: boolean }[]
  memberCount: number
  issueCount: number
}

export type Comment = {
  id: string
  body: string
  user?: UserRef | null
  createdAt: string
  editedAt?: string | null
  resolvedAt?: string | null
  replies?: Comment[]
  children?: Comment[]
}

export type Attachment = { id: string; title: string; subtitle: string | null; url: string }

export type IssueSummary = {
  id: string
  identifier: string
  title: string
  url: string
  priority: number
  priorityLabel: string
  state: StateRef | null
  assignee: UserRef | null
  dueDate: string | null
  updatedAt: string
}

export type Issue = IssueSummary & {
  description: string | null
  team: TeamRef
  creator: UserRef | null
  labels: LabelRef[]
  project: SlugRef | null
  projectMilestone: MilestoneRef | null
  cycle: CycleRef | null
  parent: IssueRef | null
  estimate: number | null
  createdAt: string
  children?: (IssueRef & { assignee: UserRef | null; priority: number })[]
  relations?: {
    blocks: IssueRef[]
    blockedBy: IssueRef[]
    related: IssueRef[]
    duplicateOf: IssueRef | null
    duplicates: IssueRef[]
  }
  attachments?: Attachment[]
  comments?: Comment[]
}

export type StatusUpdate = {
  id: string
  body: string
  health?: string | null
  user?: UserRef | null
  createdAt: string
}

export type Project = {
  id: string
  name: string
  slugId: string
  url?: string
  summary?: string | null
  description?: string | null
  content?: string | null
  status?: { name: string; type: string; color: string } | null
  lead?: UserRef | null
  startDate?: string | null
  targetDate?: string | null
  progress?: number | null
  health?: string | null
  milestones?: { id: string; name: string; targetDate: string | null; progress?: number | null }[]
  latestUpdate?: StatusUpdate | null
  createdAt?: string
  updatedAt?: string
}

export type Document = {
  id: string
  title: string
  slugId: string
  content?: string | null
  creator?: UserRef | null
  project?: SlugRef | null
  createdAt?: string
  updatedAt?: string
}

export type Initiative = {
  id: string
  name: string
  slugId: string
  description?: string | null
  content?: string | null
  status?: string | { name: string } | null
  owner?: UserRef | null
  targetDate?: string | null
  projects?: SlugRef[]
  latestUpdate?: StatusUpdate | null
  createdAt?: string
  updatedAt?: string
}

export type CustomView = {
  id: string
  name: string
  slugId: string
  description?: string | null
  owner?: UserRef | null
}

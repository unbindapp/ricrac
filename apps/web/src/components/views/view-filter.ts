// A view's filter builder: conditions (ANDed, their values ORed) to and from the server's filter
// language (filterHelp in ops/filter.go for issues, pushProjectFilter in ops/projects.go).

export type TFilterModel = "issue" | "project";
export type TFilterOp = "is" | "isNot";
export type TFilter = Record<string, unknown>;

export type TIssueField =
  | "statusType"
  | "status"
  | "assignee"
  | "creator"
  | "priority"
  | "labels"
  | "project"
  | "cycle"
  | "dueDate";

export type TProjectField = "projectStatus" | "lead" | "projectPriority" | "health";

export type TFilterField = TIssueField | TProjectField;

export type TCondition = { field: TFilterField; op: TFilterOp; values: string[] };

/** The value of a person or reference condition that stands for "nobody" / "none". */
export const NONE = "none";
/** The person who looks at the view. */
export const ME = "me";

export const FIELDS: Record<TFilterModel, TFilterField[]> = {
  issue: [
    "statusType",
    "status",
    "assignee",
    "creator",
    "priority",
    "labels",
    "project",
    "cycle",
    "dueDate",
  ],
  project: ["projectStatus", "lead", "projectPriority", "health"],
};

/** Fields that only take "is". */
export const IS_ONLY: TFilterField[] = ["dueDate"];

type TCodec = {
  encode: (op: TFilterOp, values: string[]) => TFilter;
  decode: (clause: TFilter) => Omit<TCondition, "field"> | null;
};

const isObject = (value: unknown): value is TFilter =>
  typeof value === "object" && value !== null && !Array.isArray(value);

/** The single key of an object and its value, or null. */
function only(value: unknown): [string, unknown] | null {
  if (!isObject(value)) return null;
  const entries = Object.entries(value);
  return entries.length === 1 ? entries[0] : null;
}

function strings(value: unknown): string[] | null {
  return Array.isArray(value) && value.every((item) => typeof item === "string") ? value : null;
}

/** {key: {sub: {in|nin: values}}}, e.g. {state: {type: {in: ["started"]}}}. */
function listCodec(key: string, sub: string | null, asNumbers = false): TCodec {
  const wrap = (inner: TFilter) => ({ [key]: sub ? { [sub]: inner } : inner });
  return {
    encode: (op, values) =>
      wrap({ [op === "is" ? "in" : "nin"]: asNumbers ? values.map(Number) : values }),
    decode: (clause) => {
      const outer = only(clause);
      if (!outer || outer[0] !== key) return null;
      const inner = sub ? only(outer[1]) : ["", outer[1]];
      if (!inner || (sub && inner[0] !== sub)) return null;
      const comparison = only(inner[1]);
      if (!comparison || (comparison[0] !== "in" && comparison[0] !== "nin")) return null;
      const list = comparison[1];
      if (!Array.isArray(list)) return null;
      return { op: comparison[0] === "in" ? "is" : "isNot", values: list.map(String) };
    },
  };
}

/**
 * A reference that can be empty (assignee, project, cycle): `some` and `none` build the clause
 * for its values, null the empty one. "Is not" keeps the empty ones, as people expect.
 */
function nullableCodec(
  key: string,
  match: { some: (values: string[]) => TFilter; not: (values: string[]) => TFilter },
  parse: (inner: unknown) => { values: string[]; negated: boolean } | null,
): TCodec {
  const empty = (isEmpty: boolean) => ({ [key]: { null: isEmpty } });
  const decodeNull = (clause: unknown) => {
    const outer = only(clause);
    if (!outer || outer[0] !== key) return null;
    const inner = only(outer[1]);
    if (!inner || inner[0] !== "null" || typeof inner[1] !== "boolean") return null;
    return inner[1];
  };
  const decodeMatch = (clause: unknown) => {
    const outer = only(clause);
    if (!outer || outer[0] !== key) return null;
    return parse(outer[1]);
  };
  return {
    encode: (op, values) => {
      const refs = values.filter((value) => value !== NONE);
      const hasNone = values.includes(NONE);
      if (op === "is") {
        if (refs.length === 0) return empty(true);
        if (!hasNone) return { [key]: match.some(refs) };
        return { or: [empty(true), { [key]: match.some(refs) }] };
      }
      if (refs.length === 0) return empty(false);
      if (hasNone) return { [key]: match.not(refs) };
      return { or: [empty(true), { [key]: match.not(refs) }] };
    },
    decode: (clause) => {
      const isNull = decodeNull(clause);
      if (isNull !== null) return { op: isNull ? "is" : "isNot", values: [NONE] };
      const direct = decodeMatch(clause);
      if (direct) {
        return direct.negated
          ? { op: "isNot", values: [...direct.values, NONE] }
          : { op: "is", values: direct.values };
      }
      const either = only(clause);
      const parts = either?.[0] === "or" && Array.isArray(either[1]) ? either[1] : null;
      if (!parts || parts.length !== 2 || decodeNull(parts[0]) !== true) return null;
      const matched = decodeMatch(parts[1]);
      if (!matched) return null;
      return matched.negated
        ? { op: "isNot", values: matched.values }
        : { op: "is", values: [...matched.values, NONE] };
    },
  };
}

/** {id: {in|nin: ids}}, the shape people and projects match by. */
const byId = {
  some: (values: string[]) => ({ id: { in: values } }),
  not: (values: string[]) => ({ id: { nin: values } }),
};

function parseById(inner: unknown) {
  const id = only(inner);
  if (!id || id[0] !== "id") return null;
  const comparison = only(id[1]);
  const values = comparison && strings(comparison[1]);
  if (!comparison || !values || (comparison[0] !== "in" && comparison[0] !== "nin")) return null;
  return { values, negated: comparison[0] === "nin" };
}

const cycleMatch = {
  some: () => ({ isActive: { eq: true } }),
  not: () => ({ isActive: { eq: false } }),
};

function parseCycle(inner: unknown) {
  const active = only(inner);
  if (!active || active[0] !== "isActive") return null;
  const eq = only(active[1]);
  if (!eq || eq[0] !== "eq" || typeof eq[1] !== "boolean") return null;
  return { values: ["current"], negated: !eq[1] };
}

const labelsCodec: TCodec = {
  encode: (op, values) => ({ labels: { [op === "is" ? "some" : "none"]: { id: { in: values } } } }),
  decode: (clause) => {
    const outer = only(clause);
    if (!outer || outer[0] !== "labels") return null;
    const quantifier = only(outer[1]);
    if (!quantifier || (quantifier[0] !== "some" && quantifier[0] !== "none")) return null;
    const matched = parseById(quantifier[1]);
    if (!matched || matched.negated) return null;
    return { op: quantifier[0] === "some" ? "is" : "isNot", values: matched.values };
  },
};

const dueDateCodec: TCodec = {
  encode: (_op, values) => {
    const clauses = values.map((value) => {
      if (value === "overdue") return { dueDate: { lt: "P0D" } };
      if (value === "week") return { dueDate: { lte: "P1W" } };
      return { dueDate: { null: true } };
    });
    return clauses.length === 1 ? clauses[0] : { or: clauses };
  },
  decode: (clause) => {
    const outer = only(clause);
    if (!outer) return null;
    const parts = outer[0] === "or" && Array.isArray(outer[1]) ? outer[1] : [clause];
    const values: string[] = [];
    for (const part of parts) {
      const due = only(part);
      const comparison = due?.[0] === "dueDate" ? only(due[1]) : null;
      if (!comparison) return null;
      const [op, value] = comparison;
      if (op === "lt" && value === "P0D") values.push("overdue");
      else if (op === "lte" && value === "P1W") values.push("week");
      else if (op === "null" && value === true) values.push("none");
      else return null;
    }
    return { op: "is", values };
  },
};

const workspaceTeamCodec = listCodec("team", "key");

const CODECS: Record<TFilterField, TCodec> = {
  statusType: listCodec("state", "type"),
  status: listCodec("state", "name"),
  assignee: nullableCodec("assignee", byId, parseById),
  creator: nullableCodec("creator", byId, parseById),
  priority: listCodec("priority", null, true),
  labels: labelsCodec,
  project: nullableCodec("project", byId, parseById),
  cycle: nullableCodec("cycle", cycleMatch, parseCycle),
  dueDate: dueDateCodec,
  projectStatus: listCodec("status", "type"),
  lead: nullableCodec("lead", byId, parseById),
  projectPriority: listCodec("priority", null, true),
  health: listCodec("health", null),
};

/** The filter object for a list of conditions; empty conditions are left out. */
export function conditionsToFilter(conditions: TCondition[]): TFilter {
  const clauses = conditions
    .filter((condition) => condition.values.length > 0)
    .map((condition) => CODECS[condition.field].encode(condition.op, condition.values));
  if (clauses.length === 0) return {};
  return { and: clauses };
}

/**
 * The conditions a filter stands for, or null when the builder can't show it (a hand-written filter).
 * Team clauses of older views are left out: a workspace has one team.
 */
export function filterToConditions(filter: TFilter, model: TFilterModel): TCondition[] | null {
  if (Object.keys(filter).length === 0) return [];
  const parts = only(filter)?.[0] === "and" ? filter.and : [filter];
  if (!Array.isArray(parts) || !parts.every(isObject)) return null;
  const clauses = parts.flatMap((part) =>
    Object.entries(part).map(([key, value]) => ({ [key]: value })),
  );
  const conditions: TCondition[] = [];
  for (const clause of clauses) {
    if (workspaceTeamCodec.decode(clause)) continue;
    const field = FIELDS[model].find((candidate) => CODECS[candidate].decode(clause));
    if (!field) return null;
    const decoded = CODECS[field].decode(clause);
    if (!decoded) return null;
    conditions.push({ field, ...decoded });
  }
  return conditions;
}

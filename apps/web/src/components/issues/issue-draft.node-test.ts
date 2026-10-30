import assert from "node:assert/strict";
import { test } from "node:test";

import {
  applyPatch,
  draftToCreateArgs,
  EMPTY_DRAFT,
  patchToChanges,
  patchToOptimistic,
} from "./issue-draft.ts";

const project = { id: "p1", name: "Launch", slugId: "launch" };
const bug = { id: "l1", name: "Bug", color: "#eb5757", parent: null };
const feature = { id: "l2", name: "Feature", color: "#bb87fc", parent: null };

const beta = { id: "m1", name: "Beta", targetDate: null, projectId: "p1" };
const launchDay = { id: "m2", name: "Launch day", targetDate: null, projectId: null };

test("changing the project drops the milestone of the old project", () => {
  const draft = applyPatch({ ...EMPTY_DRAFT, project, projectMilestone: beta }, { project: null });
  assert.equal(draft.projectMilestone, null);
});

test("changing the project keeps a milestone of the workspace's own", () => {
  const draft = applyPatch(
    { ...EMPTY_DRAFT, project, projectMilestone: launchDay },
    { project: null },
  );
  assert.deepEqual(draft.projectMilestone, launchDay);
});

test("a milestone picked after its project stays", () => {
  const draft = applyPatch(EMPTY_DRAFT, { project, projectMilestone: beta });
  assert.deepEqual(draft.projectMilestone, beta);
});

test("an optimistic change only touches the milestone when the patch does", () => {
  assert.equal("projectMilestone" in patchToOptimistic({ project: null }), false);
  assert.equal(patchToOptimistic({ project: null, projectMilestone: null }).projectMilestone, null);
});

test("a new issue only sends the properties that were set", () => {
  const args = draftToCreateArgs({
    teamId: "t1",
    title: "  Fix login  ",
    description: " ",
    draft: { ...EMPTY_DRAFT, priority: 1, labels: [bug] },
  });
  assert.deepEqual(args, { team: "t1", title: "Fix login", priority: 1, labels: ["l1"] });
});

test("clearing a property sends null", () => {
  assert.deepEqual(patchToChanges({ assignee: null, dueDate: null, parent: null }), {
    assignee: null,
    dueDate: null,
    parentId: null,
  });
});

test("labels go as additions and removals against the current ones", () => {
  assert.deepEqual(patchToChanges({ labels: [feature] }, [bug]), {
    addLabels: ["l2"],
    removeLabels: ["l1"],
  });
  assert.deepEqual(patchToChanges({ labels: [bug] }, [bug]), {});
  assert.deepEqual(patchToChanges({ labels: [bug] }), { labels: ["l1"] });
});

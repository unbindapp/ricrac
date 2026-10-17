// Fills a development server with a workspace to click through:
// node scripts/seed.mjs "$(cd ../server && go run . session ann@acme.test)" [http://localhost:3000]
const [cookie, base = "http://localhost:3000"] = process.argv.slice(2);
if (!cookie) {
  console.error("Pass a session cookie: node scripts/seed.mjs <cookie> [server]");
  process.exit(1);
}

async function request(method, path, body, workspace) {
  const response = await fetch(base + path, {
    method,
    headers: {
      cookie,
      "x-ricrac": "1",
      "content-type": "application/json",
      ...(workspace ? { "x-ricrac-workspace": workspace } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await response.json().catch(() => null);
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(data)}`);
  return data;
}

const call = (name, args) => request("POST", `/v1/ops/${name}`, args, "acme");

await request("POST", "/api/workspaces", { name: "Acme", urlKey: "acme", teamName: "Engineering", teamKey: "ENG" }).catch(
  (error) => console.log("workspace exists:", error.message),
);
await call("save_team", { name: "Design", key: "DES" }).catch(() => {});
const project = await call("save_project", { name: "Launch v2", setTeams: ["ENG", "DES"], targetDate: "2030-03-31" }).catch(
  () => null,
);
const titles = [
  ["Fix login redirect loop", "started", 1, ["Bug"]],
  ["Add dark mode to settings", "unstarted", 3, ["Feature"]],
  ["Crash when uploading large files", "started", 2, ["Bug"]],
  ["Improve onboarding copy", "backlog", 4, ["Improvement"]],
  ["Billing page shows wrong currency", "completed", 2, ["Bug"]],
  ["Keyboard shortcuts for the issue list", "backlog", 0, ["Feature"]],
  ["Search is slow on large workspaces", "unstarted", 2, ["Improvement"]],
  ["Export issues to CSV", "backlog", 3, ["Feature"]],
  ["Mobile layout breaks on small screens", "started", 1, ["Bug"]],
  ["Rate limit the public API", "canceled", 3, []],
];
for (const [title, state, priority, labels] of titles) {
  await call("save_issue", {
    team: "ENG",
    title,
    state,
    priority,
    labels,
    assignee: priority % 2 ? "me" : null,
    project: project && priority < 3 ? project.id : null,
    description: `## Context\n\n${title}.\n\n- [ ] Reproduce\n- [ ] Fix\n`,
  });
}
await call("save_comment", { issueId: "ENG-1", body: "Happens on Safari only, I think." });
console.log("Seeded http://localhost:5173/acme");

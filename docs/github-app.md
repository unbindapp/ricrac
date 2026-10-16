# The GitHub App

Ricrac connects to GitHub as a GitHub App. You create the app once for your Ricrac server; then
workspace admins press **Connect GitHub** in the integration settings (or call `connect_github`),
install the app on their GitHub account or organization, and pick repositories.

With the app installed:

- Pull requests, branches and commits that mention an identifier like `ENG-123` (in the branch
  name, the pull request's title or body, or the commit message) are attached to the issue. An
  opened pull request or a pushed branch moves the issue to the team's "pull request opened"
  status (the first started status unless the team picks one) when nobody started it yet. A
  merged pull request that says `fixes`, `closes` or `resolves ENG-123` moves it to the "merged"
  status (the first completed status unless the team picks one); a commit saying so does that
  only once it is on the default branch.
- Repositories synced with a team (`save_github_repo_sync`) keep their issues in step both ways:
  issues opened, edited, labeled, assigned, closed, reopened, deleted or commented on GitHub
  change the Ricrac issue, and title, description, status, labels, assignee and comments changed
  in Ricrac go back to GitHub. Existing issues are imported when the sync is created.
- People link their GitHub account in their settings (or when they install the app). GitHub
  authors and assignees map to them; others show as `@login on GitHub`.

## Creating the app

Replace `https://ricrac.example.com` with your `PUBLIC_URL`.

### In one step, from a manifest

Save this page as `create-github-app.html`, open it in a browser while signed in to GitHub and
press the button. To create the app under an organization instead of your account, change the
form's action to `https://github.com/organizations/<org>/settings/apps/new`.

```html
<form action="https://github.com/settings/apps/new" method="post">
  <input type="hidden" name="manifest" id="manifest">
  <button>Create the Ricrac GitHub App</button>
</form>
<script>
  document.getElementById("manifest").value = JSON.stringify({
    "name": "Ricrac",
    "url": "https://ricrac.example.com",
    "description": "Links pull requests, branches and commits to Ricrac issues and syncs repository issues with Ricrac teams.",
    "public": true,
    "hook_attributes": { "url": "https://ricrac.example.com/github/webhook", "active": true },
    "redirect_url": "https://ricrac.example.com/",
    "callback_urls": [
      "https://ricrac.example.com/api/github/setup",
      "https://ricrac.example.com/api/github/callback"
    ],
    "setup_url": "https://ricrac.example.com/api/github/setup",
    "setup_on_update": true,
    "request_oauth_on_install": true,
    "default_permissions": {
      "metadata": "read",
      "contents": "read",
      "issues": "write",
      "pull_requests": "read"
    },
    "default_events": ["issues", "issue_comment", "pull_request", "push"]
  })
</script>
```

GitHub creates the app and sends you to `https://ricrac.example.com/?code=...`. Within an hour,
exchange that code for the app's settings:

```sh
curl -X POST -H "Accept: application/vnd.github+json" https://api.github.com/app-manifests/<code>/conversions
```

The answer has everything Ricrac needs:

| Field | Variable |
| --- | --- |
| `id` | `GITHUB_APP_ID` |
| `slug` | `GITHUB_APP_SLUG` |
| `pem` | `GITHUB_APP_PRIVATE_KEY` |
| `webhook_secret` | `GITHUB_WEBHOOK_SECRET` |
| `client_id` | `GITHUB_CLIENT_ID` |
| `client_secret` | `GITHUB_CLIENT_SECRET` |

### By hand

In GitHub, **Settings → Developer settings → GitHub Apps → New GitHub App** (or the same under an
organization's settings):

| Setting | Value |
| --- | --- |
| GitHub App name | Ricrac (or anything; its URL name is the slug) |
| Homepage URL | `https://ricrac.example.com` |
| Callback URL | `https://ricrac.example.com/api/github/setup`, and add `https://ricrac.example.com/api/github/callback` |
| Expire user authorization tokens | either; Ricrac only uses a person's token once |
| Request user authorization (OAuth) during installation | **on** |
| Enable Device Flow | off |
| Setup URL | `https://ricrac.example.com/api/github/setup` (GitHub greys it out while the option above is on and uses the callback URL; both work) |
| Redirect on update | on |
| Webhook | Active |
| Webhook URL | `https://ricrac.example.com/github/webhook` |
| Webhook secret | a long random string, also set as `GITHUB_WEBHOOK_SECRET` |
| Repository permissions | Metadata: Read-only, Contents: Read-only, Issues: Read and write, Pull requests: Read-only |
| Subscribe to events | Issues, Issue comment, Pull request, Push |
| Where can this GitHub App be installed? | Any account, unless every workspace's repositories live under the app's owner |

GitHub always sends the **Installation** and **Installation repositories** events to apps; there
is no checkbox for them. Ricrac uses them to forget uninstalled or suspended installations and
repositories taken away from the app.

After creating it:

1. Note the **App ID** (`GITHUB_APP_ID`) and the slug in the app's public link
   `https://github.com/apps/<slug>` (`GITHUB_APP_SLUG`).
2. Note the **Client ID** (`GITHUB_CLIENT_ID`) and generate a client secret
   (`GITHUB_CLIENT_SECRET`).
3. Generate a private key and set the `.pem` file's contents as `GITHUB_APP_PRIVATE_KEY`. Where a
   variable can't hold line breaks, write them as `\n`.

## Configuring Ricrac

Set the six variables on the server (on Unbind, deploy the code first, then set them: every
variable change restarts the pod). `GITHUB_API_URL` and `GITHUB_WEB_URL` only change for GitHub
Enterprise Server (`https://<host>/api/v3` and `https://<host>`).

Then, as a workspace admin:

1. **Connect GitHub** (`connect_github`) and install the app on the account or organization,
   picking repositories. GitHub asks you to authorize the app, which tells Ricrac who you are:
   the installation is only connected when your GitHub account can reach it, and your GitHub
   login is linked to your Ricrac account.
2. Optionally pick each team's statuses for opened and merged pull requests
   (`save_team` with `githubPrOpenedState` and `githubPrMergedState`).
3. To sync a repository's issues with a team, `save_github_repo_sync` with the repository and the
   team (and a project, if issues from GitHub should join one). `includeClosed` imports closed
   issues too. `list_github_repo_syncs` shows how the import went and the last error.

Everyone else links their GitHub account from their settings (`/api/github/connect`), so their
pull requests, comments and assignments on GitHub show as theirs.

## What goes where

- Changes made in Ricrac are queued in the same transaction and sent within seconds by a
  background job, with the installation's token. Failures are retried after 1, 5 and 30 minutes
  and 2 hours, then dropped; the last error shows on the repository sync.
- Comments from Ricrac appear on GitHub as the app, starting with `**<name> on Ricrac**`.
- Events caused by the app itself (its bot account) are ignored, and changes that came from
  GitHub aren't sent back, so nothing echoes.
- Issue events older than what Ricrac already has (by GitHub's `updated_at`) are ignored.

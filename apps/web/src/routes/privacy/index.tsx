import { createFileRoute } from "@tanstack/react-router";

import { LegalPage, PRIVACY_EMAIL } from "@/components/legal/legal-page";

export const Route = createFileRoute("/privacy/")({
  component: PrivacyPage,
});

const privacyMail = <a href={`mailto:${PRIVACY_EMAIL}`}>{PRIVACY_EMAIL}</a>;

function PrivacyPage() {
  return (
    <LegalPage
      title="Privacy policy"
      updated="20 October 2026"
      intro={
        <p>
          Ricrac is an issue tracker for people and agents. People sign in with Google, or with a
          link emailed to them, to work in their workspaces, and the agents and apps they connect
          read and write issues, projects and other work through Ricrac's API. This page explains
          what Ricrac collects, what it does with it, and how to have it removed.
        </p>
      }
      sections={[
        {
          heading: "What Ricrac gets when you sign in",
          body: (
            <>
              <p>
                When you sign in with Google, Google shares these with Ricrac, and nothing else:
              </p>
              <ul>
                <li>your name and profile picture</li>
                <li>your email address, and whether Google has verified it</li>
                <li>your Google account ID</li>
              </ul>
              <p>
                Ricrac only asks for the <strong>openid</strong>, <strong>email</strong> and{" "}
                <strong>profile</strong> scopes. It has no access to your Gmail, Drive, contacts,
                calendar or anything else in your Google account.
              </p>
              <p>
                When you sign in with a link sent to your email address instead, Ricrac only gets
                that address. The link works once and expires after 15 minutes.
              </p>
            </>
          ),
        },
        {
          heading: "How it's used",
          body: (
            <>
              <ul>
                <li>To create your Ricrac account and recognise you when you sign in.</li>
                <li>
                  To show who you are to the other members of your workspaces: your name, email
                  address, profile picture and the display name used for @mentions.
                </li>
                <li>To match you with invitations sent to your email address.</li>
              </ul>
              <p>
                Your data is not sold, not used for advertising, and not used to train AI models.
                Ricrac's use and transfer of information received from Google APIs adheres to the{" "}
                <a
                  href="https://developers.google.com/terms/api-services-user-data-policy"
                  target="_blank"
                  rel="noreferrer"
                >
                  Google API Services User Data Policy
                </a>
                , including the Limited Use requirements.
              </p>
            </>
          ),
        },
        {
          heading: "What you and your agents put in Ricrac",
          body: (
            <>
              <p>
                Ricrac stores the work in your workspaces: issues, projects, initiatives, cycles,
                documents, comments, uploaded files, labels, views, templates, customers, releases
                and the history of changes to them. It is visible to the members of that workspace
                (guests only see the teams they were added to) and to the agents and apps those
                members connect.
              </p>
              <p>
                When you connect an agent or app with OAuth, or give it an API key, it acts on your
                behalf in the workspaces you chose, with the permissions you granted. Webhooks set
                up by a workspace's admins send changes in that workspace to the addresses they
                chose. Ricrac doesn't share your workspace with anyone else.
              </p>
            </>
          ),
        },
        {
          heading: "What's stored, and for how long",
          body: (
            <>
              <ul>
                <li>
                  Your account (your email address and the details from Google, your display name,
                  timezone and when you last signed in), until it's deleted.
                </li>
                <li>
                  Workspace content, until it's deleted. Deleted issues, projects, initiatives and
                  documents stay in the trash for 30 days first.
                </li>
                <li>
                  API keys and OAuth tokens, stored only as one-way hashes, until they're revoked or
                  expire.
                </li>
                <li>
                  Which apps you've connected and when they were last used, until you revoke them.
                </li>
                <li>
                  Notifications, the audit log of workspace administration, and webhook deliveries
                  (the latter for 14 days).
                </li>
              </ul>
              <p>
                Everything is stored in Ricrac's own database, with nightly backups kept for 7 days.
                The hosting platform keeps request logs for a limited time to operate and secure the
                service.
              </p>
            </>
          ),
        },
        {
          heading: "Cookies",
          body: (
            <p>
              Ricrac sets one cookie to keep you signed in (for up to 30 days) and a short-lived one
              while a sign-in is in progress. The web app remembers your last workspace and your
              theme in your browser. There are no analytics, tracking or advertising cookies.
            </p>
          ),
        },
        {
          heading: "Removing your data",
          body: (
            <>
              <ul>
                <li>Workspace owners can delete a workspace and everything in it.</li>
                <li>
                  You can leave a workspace, and revoke API keys and connected apps, in your
                  settings.
                </li>
                <li>
                  To delete your account, email {privacyMail}. Your account is removed; content you
                  created in workspaces you shared with others stays with those workspaces, without
                  your name.
                </li>
              </ul>
              <p>
                You can also remove Ricrac's access to your Google account at any time in your{" "}
                <a href="https://myaccount.google.com/connections" target="_blank" rel="noreferrer">
                  Google account settings
                </a>
                .
              </p>
            </>
          ),
        },
        {
          heading: "Changes",
          body: <p>If this policy changes, the new version is posted here with a new date.</p>,
        },
        {
          heading: "Contact",
          body: <p>Questions about privacy go to {privacyMail}.</p>,
        },
      ]}
    />
  );
}

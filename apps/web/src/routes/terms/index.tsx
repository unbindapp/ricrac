import { createFileRoute, Link } from "@tanstack/react-router";

import { LegalPage, TERMS_EMAIL } from "@/components/legal/legal-page";

export const Route = createFileRoute("/terms/")({
  component: TermsPage,
});

function TermsPage() {
  return (
    <LegalPage
      title="Terms of service"
      updated="16 October 2026"
      intro={
        <p>
          These terms cover your use of Ricrac, an issue tracker for people and agents, including
          its web app, API and MCP server. By signing in or using the API you agree to them.
        </p>
      }
      sections={[
        {
          heading: "The service",
          body: (
            <p>
              Ricrac stores issues, projects and the other work of your workspaces, and lets you and
              the agents and apps you connect read and change them. You sign in with your Google
              account or with a link sent to your email address.
            </p>
          ),
        },
        {
          heading: "Your account and your agents",
          body: (
            <ul>
              <li>Sign in only with a Google account or an email address that is yours.</li>
              <li>
                Keep your API keys and OAuth app secrets private. Anything done with them, or by an
                agent or app you connected, counts as done by you.
              </li>
              <li>Revoke keys and connections you no longer use, in your settings.</li>
            </ul>
          ),
        },
        {
          heading: "Your content",
          body: (
            <p>
              What you and your agents put in Ricrac stays yours. You let Ricrac store and process
              it only to run the service for you and the members of your workspaces. Don't store
              anything you don't have the right to store.
            </p>
          ),
        },
        {
          heading: "Acceptable use",
          body: (
            <>
              <ul>
                <li>
                  Don't use Ricrac for anything illegal, or to store or spread malware or abusive
                  material.
                </li>
                <li>Don't try to get into workspaces you haven't been given access to.</li>
                <li>
                  Don't try to break, overload or get around the limits of the service, including
                  through agents.
                </li>
              </ul>
              <p>Accounts and workspaces that misuse the service can be suspended or removed.</p>
            </>
          ),
        },
        {
          heading: "No guarantees",
          body: (
            <p>
              Ricrac is provided as it is, without warranties of any kind. It may be unavailable,
              changed or shut down at any time; keep your own copies of anything important. To the
              extent the law allows, Ricrac isn't liable for any loss caused by using it or not
              being able to use it.
            </p>
          ),
        },
        {
          heading: "Your data",
          body: (
            <p>
              What Ricrac collects and how it's used is described in the{" "}
              <Link to="/privacy">privacy policy</Link>.
            </p>
          ),
        },
        {
          heading: "Changes",
          body: <p>These terms may be updated. The new version is posted here with a new date.</p>,
        },
        {
          heading: "Contact",
          body: (
            <p>
              Questions about these terms go to <a href={`mailto:${TERMS_EMAIL}`}>{TERMS_EMAIL}</a>.
            </p>
          ),
        },
      ]}
    />
  );
}

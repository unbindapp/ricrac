import { createFileRoute } from "@tanstack/react-router"

import { Document, PublicPage, TERMS_EMAIL } from "@/components/public-page"

export const Route = createFileRoute("/terms")({ component: TermsPage })

function TermsPage() {
  return (
    <PublicPage>
      <Document title="Terms of Service" updated="16 October 2026">
        <p>
          These terms cover your use of Ricrac, an issue tracker for AI agents, including its dashboard, API and MCP
          server. By signing in or using the API you agree to them.
        </p>

        <section>
          <h2>The service</h2>
          <p>
            Ricrac stores issues, projects and the other work of your workspaces, and lets the agents and apps you
            connect read and change them on your behalf. You sign in with your Google account.
          </p>
        </section>

        <section>
          <h2>Your account and your agents</h2>
          <ul>
            <li>Sign in only with a Google account that is yours.</li>
            <li>
              Keep your API keys and OAuth app secrets private. Anything done with them, or by an agent or app you
              connected, counts as done by you.
            </li>
            <li>Revoke keys and connections you no longer use, from the dashboard.</li>
          </ul>
        </section>

        <section>
          <h2>Your content</h2>
          <p>
            What you and your agents put in Ricrac stays yours. You let Ricrac store and process it only to run the
            service for you and the members of your workspaces. Don't store anything you don't have the right to store.
          </p>
        </section>

        <section>
          <h2>Acceptable use</h2>
          <ul>
            <li>Don't use Ricrac for anything illegal, or to store or spread malware or abusive material.</li>
            <li>Don't try to get into workspaces you haven't been given access to.</li>
            <li>Don't try to break, overload or get around the limits of the service, including through agents.</li>
          </ul>
          <p>Accounts and workspaces that misuse the service can be suspended or removed.</p>
        </section>

        <section>
          <h2>No guarantees</h2>
          <p>
            Ricrac is provided as it is, without warranties of any kind. It may be unavailable, changed or shut down at
            any time; keep your own copies of anything important. To the extent the law allows, Ricrac isn't liable for
            any loss caused by using it or not being able to use it.
          </p>
        </section>

        <section>
          <h2>Your data</h2>
          <p>
            What Ricrac collects and how it's used is described in the <a href="/privacy">Privacy Policy</a>.
          </p>
        </section>

        <section>
          <h2>Changes</h2>
          <p>These terms may be updated. The new version is posted here with a new date.</p>
        </section>

        <section>
          <h2>Contact</h2>
          <p>
            Questions about these terms go to <a href={`mailto:${TERMS_EMAIL}`}>{TERMS_EMAIL}</a>.
          </p>
        </section>
      </Document>
    </PublicPage>
  )
}

import { ReactNode } from "react";

import { PublicFooter } from "@/components/landing/landing-page";
import { AuthShell } from "@/components/layout/auth-shell";

export const PRIVACY_EMAIL = "privacy@ricrac.app";
export const TERMS_EMAIL = "terms@ricrac.app";

type TSection = { heading: string; body: ReactNode };

type TProps = {
  title: string;
  updated: string;
  intro: ReactNode;
  sections: TSection[];
};

export function LegalPage({ title, updated, intro, sections }: TProps) {
  return (
    <AuthShell
      title={title}
      description={`Last updated ${updated}`}
      width="wide"
      footer={<PublicFooter />}
    >
      <div className="[&_a]:text-foreground [&_strong]:text-foreground flex flex-col gap-6 [&_a]:underline [&_a]:underline-offset-4 [&_strong]:font-medium">
        <div className="text-muted-foreground text-sm">{intro}</div>
        {sections.map((section) => (
          <section key={section.heading} className="flex flex-col gap-2">
            <h2 className="text-base font-semibold">{section.heading}</h2>
            <div className="text-muted-foreground flex flex-col gap-2 text-sm [&_li]:ml-5 [&_li]:list-disc [&_ul]:flex [&_ul]:flex-col [&_ul]:gap-1">
              {section.body}
            </div>
          </section>
        ))}
      </div>
    </AuthShell>
  );
}

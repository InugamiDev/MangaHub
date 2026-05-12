import Link from "next/link";
import { docs } from "@/lib/docs";

export default function DocsIndexPage() {
  return (
    <main className="shell docs-layout">
      <aside className="docs-nav">
        {docs.map((doc) => (
          <Link href={`/docs/${doc.slug}`} key={doc.slug}>
            {doc.title}
          </Link>
        ))}
      </aside>
      <section>
        <div className="section-head">
          <div>
            <span className="eyebrow">PDF-derived project pack</span>
            <h1 className="section-title">MangaHub docs</h1>
          </div>
          <p className="section-copy">
            Requirements, architecture, API contracts, protocol behavior, use cases, CLI reference, demo script, roadmap, and AI usage policy.
          </p>
        </div>
        <div className="grid-2">
          {docs.map((doc) => (
            <Link className="doc-card" href={`/docs/${doc.slug}`} key={doc.slug}>
              <h3>{doc.title}</h3>
              <p>{doc.description}</p>
            </Link>
          ))}
        </div>
      </section>
    </main>
  );
}

import Link from "next/link";
import { notFound } from "next/navigation";
import { MarkdownView } from "@/components/MarkdownView";
import { docs, loadDoc } from "@/lib/docs";

export function generateStaticParams() {
  return docs.map((doc) => ({ slug: doc.slug }));
}

export default async function DocPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const doc = docs.find((item) => item.slug === slug);
  if (!doc) {
    notFound();
  }
  return (
    <main className="shell docs-layout">
      <aside className="docs-nav">
        {docs.map((item) => (
          <Link href={`/docs/${item.slug}`} key={item.slug}>
            {item.title}
          </Link>
        ))}
      </aside>
      <MarkdownView content={loadDoc(slug)} />
    </main>
  );
}

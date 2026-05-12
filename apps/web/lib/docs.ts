import fs from "node:fs";
import path from "node:path";

export const docs = [
  { slug: "setup", title: "Setup Instructions", description: "Local and production setup for Neon, R2, API, web, migrations, and smoke tests." },
  { slug: "requirements", title: "Requirements", description: "Objectives, deliverables, grading, data requirements, and success metrics." },
  { slug: "architecture", title: "Architecture", description: "Services, ports, data flow, database, and integration map." },
  { slug: "api", title: "API Documentation", description: "HTTP, admin, source, chapter, storage, auth, and protocol API reference." },
  { slug: "production", title: "Production Setup", description: "Neon/Postgres, R2 storage, legal metadata policy, migrations, and checks." },
  { slug: "protocols", title: "Protocols", description: "TCP, UDP, gRPC, and WebSocket runtime behavior." },
  { slug: "use-cases", title: "Use Cases", description: "UC-001 through UC-031 grouped by feature area." },
  { slug: "cli-reference", title: "CLI Reference", description: "Command behavior distilled from the MangaHub CLI manual." },
  { slug: "demo-script", title: "Demo Script", description: "Step-by-step demo flow and acceptance checklist." },
  { slug: "roadmap", title: "Roadmap", description: "10-12 week implementation plan and bonus feature strategy." },
  { slug: "ai-usage", title: "AI Usage", description: "Course-safe AI usage statement and boundaries." },
];

export function loadDoc(slug: string): string {
  const baseCandidates = [
    path.join(process.cwd(), "..", "..", "docs"),
    path.join(process.cwd(), "docs"),
  ];
  for (const base of baseCandidates) {
    const file = path.join(base, `${slug}.md`);
    if (fs.existsSync(file)) {
      return fs.readFileSync(file, "utf8");
    }
  }
  return `# Missing document\n\nThe document \`${slug}.md\` could not be found.`;
}

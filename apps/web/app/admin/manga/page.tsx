import { AdminMangaList } from "@/components/admin/AdminMangaList";
import { PageTitle } from "@/components/catalog/PageTitle";
import { loadCatalog } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function AdminMangaPage() {
  const catalog = await loadCatalog();
  return (
    <main className="admin-surface">
      <div className="shell">
        <PageTitle eyebrow="Admin" title="Manga Management" copy="Create, search, delete, and open chapter management for backend catalog records." />
        <AdminMangaList catalog={catalog} />
      </div>
    </main>
  );
}

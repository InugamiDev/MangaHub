import { MangaCreateForm } from "@/components/admin/MangaCreateForm";
import { PageTitle } from "@/components/catalog/PageTitle";

export default function AdminNewMangaPage() {
  return (
    <main className="admin-surface">
      <div className="shell">
        <PageTitle eyebrow="Admin" title="Create Manga" copy="Add a catalog record through the admin API." />
        <MangaCreateForm />
      </div>
    </main>
  );
}

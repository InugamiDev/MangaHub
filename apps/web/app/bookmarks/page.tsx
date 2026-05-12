import { PageTitle } from "@/components/catalog/PageTitle";
import { UserLibraryPanel } from "@/components/user/UserLibraryPanel";

export default function BookmarksPage() {
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="User area" title="Bookmarks" copy="Saved manga, current progress, and quick continue actions." />
        <UserLibraryPanel mode="bookmarks" />
      </div>
    </main>
  );
}

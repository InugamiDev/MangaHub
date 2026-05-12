import { PageTitle } from "@/components/catalog/PageTitle";
import { UserLibraryPanel } from "@/components/user/UserLibraryPanel";

export default function HistoryPage() {
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="User area" title="Reading History" copy="Recently read titles and one-tap continue controls." />
        <UserLibraryPanel mode="history" />
      </div>
    </main>
  );
}

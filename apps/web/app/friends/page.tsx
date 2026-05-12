import { FriendsPanel } from "@/components/community/FriendsPanel";
import { PageTitle } from "@/components/catalog/PageTitle";

export default function FriendsPage() {
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Community" title="Friends and Activity" copy="Send friend requests, approve readers, and view recent completions and reviews from accepted friends." />
        <FriendsPanel />
      </div>
    </main>
  );
}

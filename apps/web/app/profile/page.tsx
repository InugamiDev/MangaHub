import { PageTitle } from "@/components/catalog/PageTitle";
import { ProfileSummary } from "@/components/user/ProfileSummary";

export default function ProfilePage() {
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Account" title="Reader Profile" copy="Profile overview, saved titles, reading activity, and account settings." />
        <ProfileSummary />
      </div>
    </main>
  );
}

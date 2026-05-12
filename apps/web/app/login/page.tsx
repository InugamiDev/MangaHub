import { AuthForm } from "@/components/auth/AuthForm";

export default function LoginPage() {
  return (
    <main className="auth-shell">
      <AuthForm mode="login" />
    </main>
  );
}

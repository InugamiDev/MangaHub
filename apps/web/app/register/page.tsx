import { AuthForm } from "@/components/auth/AuthForm";

export default function RegisterPage() {
  return (
    <main className="auth-shell">
      <AuthForm mode="register" />
    </main>
  );
}

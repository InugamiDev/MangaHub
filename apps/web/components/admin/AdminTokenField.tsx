"use client";

import { useEffect, useState } from "react";
import { Eye, EyeOff, KeyRound, ShieldCheck } from "lucide-react";

export const ADMIN_TOKEN_STORAGE_KEY = "mangahub_admin_token";
const USER_TOKEN_STORAGE_KEY = "mangahub_token";
const USER_STORAGE_KEY = "mangahub_user";

export type AdminCredentials = {
  token?: string;
  adminToken?: string;
};

export function readAdminToken() {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(ADMIN_TOKEN_STORAGE_KEY)?.trim() ?? "";
}

export function readAdminCredentials(): AdminCredentials {
  if (typeof window === "undefined") return {};
  const token = window.localStorage.getItem(USER_TOKEN_STORAGE_KEY)?.trim() ?? "";
  const adminToken = readAdminToken();
  return {
    ...(token ? { token } : {}),
    ...(adminToken ? { adminToken } : {}),
  };
}

export function hasAdminCredentials(credentials: AdminCredentials) {
  return Boolean(credentials.token || credentials.adminToken);
}

export function AdminTokenField() {
  const [token, setToken] = useState("");
  const [visible, setVisible] = useState(false);
  const [signedInUser, setSignedInUser] = useState<{ username?: string; role?: string } | null>(null);

  useEffect(() => {
    setToken(readAdminToken());
    try {
      const raw = window.localStorage.getItem(USER_STORAGE_KEY);
      setSignedInUser(raw ? JSON.parse(raw) : null);
    } catch {
      setSignedInUser(null);
    }
  }, []);

  function updateToken(value: string) {
    setToken(value);
    if (value.trim()) {
      window.localStorage.setItem(ADMIN_TOKEN_STORAGE_KEY, value.trim());
    } else {
      window.localStorage.removeItem(ADMIN_TOKEN_STORAGE_KEY);
    }
  }

  return (
    <div className="admin-token-field">
      <ShieldCheck size={16} />
      <span className="admin-auth-state">
        {signedInUser?.role === "admin" ? `Admin: ${signedInUser.username}` : "Admin login or token required"}
      </span>
      <KeyRound size={16} />
      <label>
        Server token
        <input
          aria-label="Admin sync token fallback"
          autoComplete="off"
          onChange={(event) => updateToken(event.target.value)}
          placeholder="Optional fallback"
          type={visible ? "text" : "password"}
          value={token}
        />
      </label>
      <button type="button" onClick={() => setVisible((value) => !value)} aria-label={visible ? "Hide admin token" : "Show admin token"}>
        {visible ? <EyeOff size={15} /> : <Eye size={15} />}
      </button>
    </div>
  );
}

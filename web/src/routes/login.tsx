import { useState } from "react";
import { Button } from "@/components/gh/Button";
import { useAuthStore } from "@/data/auth-store";

export function Login() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const login = useAuthStore((s) => s.loginWithPassword);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await login(username, password);
      window.location.assign("/pages/default");
    } catch {
      setError("Invalid credentials");
    }
  }

  return (
    <div style={{ display: "flex", minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
      <form onSubmit={onSubmit} style={{ display: "flex", flexDirection: "column", gap: 8, width: 320 }}>
        <h1>Sign in to gohome</h1>
        <input placeholder="username" value={username} onChange={e => setUsername(e.target.value)} />
        <input placeholder="password" type="password" value={password} onChange={e => setPassword(e.target.value)} />
        {error && <span style={{ color: "red" }}>{error}</span>}
        <Button type="submit">Sign in</Button>
      </form>
    </div>
  );
}

import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { extractApiError } from "../api/errors";
import { useAuth } from "../contexts/AuthContext";

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(email, password, orgSlug);
      navigate("/app/assessments");
    } catch (err: unknown) {
      setError(
        extractApiError(err, "Login failed. Please check your credentials."),
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <p className="mb-2 text-xs font-bold uppercase tracking-[0.2em] text-brand-400">
          Readinova
        </p>
        <h1 className="mb-8 text-2xl font-semibold">Sign in</h1>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="label mb-1" htmlFor="org-slug">
              Organisation slug
            </label>
            <input
              id="org-slug"
              type="text"
              autoComplete="organization"
              required
              className="input"
              placeholder="my-company"
              value={orgSlug}
              onChange={(e) => setOrgSlug(e.target.value)}
            />
          </div>

          <div>
            <label className="label mb-1" htmlFor="email">
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              required
              className="input"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>

          <div>
            <label className="label mb-1" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              className="input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          {error && (
            <p className="rounded-md bg-red-900/40 px-3 py-2 text-sm text-red-400">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="btn-primary w-full"
          >
            {submitting ? "Signing in…" : "Sign in"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-slate-500">
          No account?{" "}
          <Link to="/signup" className="text-brand-400 hover:underline">
            Create organisation
          </Link>
        </p>
      </div>
    </div>
  );
}

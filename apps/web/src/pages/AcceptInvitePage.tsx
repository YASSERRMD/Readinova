import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { extractApiError } from "../api/errors";
import { orgApi } from "../api/org";

export function AcceptInvitePage() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    if (password.length < 12) {
      setError("Password must be at least 12 characters.");
      return;
    }
    if (!token) {
      setError("Invalid invitation link.");
      return;
    }

    setSubmitting(true);
    try {
      await orgApi.acceptInvitation(token, email, password);
      navigate("/login");
    } catch (err: unknown) {
      setError(extractApiError(err, "Invalid or expired invitation."));
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
        <h1 className="mb-2 text-2xl font-semibold">Accept invitation</h1>
        <p className="mb-8 text-sm text-slate-400">
          Create your account to join the organisation.
        </p>

        <form onSubmit={handleSubmit} className="space-y-4">
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
              <span className="ml-1 text-xs text-slate-500">
                (min 12 chars)
              </span>
            </label>
            <input
              id="password"
              type="password"
              autoComplete="new-password"
              required
              minLength={12}
              className="input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          <div>
            <label className="label mb-1" htmlFor="confirm">
              Confirm password
            </label>
            <input
              id="confirm"
              type="password"
              autoComplete="new-password"
              required
              className="input"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
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
            {submitting ? "Creating account…" : "Accept invitation"}
          </button>
        </form>
      </div>
    </div>
  );
}

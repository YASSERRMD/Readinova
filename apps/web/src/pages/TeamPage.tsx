import { useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { orgApi } from "../api/org";

const ROLES = ["admin", "viewer", "executive", "cio", "risk", "ops"];

export function TeamPage() {
  const { user } = useAuth();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("viewer");
  const [inviteUrl, setInviteUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const canInvite = user?.role === "owner" || user?.role === "admin";

  async function handleInvite(e: React.FormEvent) {
    e.preventDefault();
    if (!user?.orgId) return;
    setError(null);
    setInviteUrl(null);
    setSubmitting(true);
    try {
      const res = await orgApi.invite(user.orgId, { email, role });
      setInviteUrl(`${window.location.origin}/accept/${res.data.token}`);
      setEmail("");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data
          ?.error ?? "Failed to send invitation.";
      setError(msg);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="max-w-xl">
      <h2 className="text-xl font-semibold">Team</h2>
      <p className="mt-1 text-sm text-slate-400">
        Manage members and send invitations.
      </p>

      {canInvite && (
        <div className="card mt-8">
          <h3 className="mb-4 text-sm font-semibold text-slate-200">
            Invite a new member
          </h3>
          <form onSubmit={handleInvite} className="space-y-4">
            <div>
              <label className="label mb-1" htmlFor="inv-email">
                Email address
              </label>
              <input
                id="inv-email"
                type="email"
                required
                className="input"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div>
              <label className="label mb-1" htmlFor="inv-role">
                Role
              </label>
              <select
                id="inv-role"
                className="input"
                value={role}
                onChange={(e) => setRole(e.target.value)}
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
            </div>

            {error && (
              <p className="rounded-md bg-red-900/40 px-3 py-2 text-sm text-red-400">
                {error}
              </p>
            )}

            {inviteUrl && (
              <div className="rounded-md bg-brand-900/40 px-3 py-2">
                <p className="mb-1 text-xs text-brand-300 font-medium">
                  Invitation link
                </p>
                <p className="break-all font-mono text-xs text-brand-200">
                  {inviteUrl}
                </p>
              </div>
            )}

            <button type="submit" disabled={submitting} className="btn-primary">
              {submitting ? "Sending…" : "Send invitation"}
            </button>
          </form>
        </div>
      )}

      {!canInvite && (
        <div className="card mt-8">
          <p className="text-sm text-slate-400">
            Only owners and admins can invite new members.
          </p>
        </div>
      )}
    </div>
  );
}

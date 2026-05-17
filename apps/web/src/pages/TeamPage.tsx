import { useEffect, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { orgApi } from "../api/org";
import { apiClient } from "../api/client";

const ROLES = ["admin", "viewer", "executive", "cio", "risk", "ops"];

interface Member {
  user_id: string;
  email: string;
  role: string;
  last_login_at?: string;
}

export function TeamPage() {
  const { user } = useAuth();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("viewer");
  const [inviteUrl, setInviteUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [members, setMembers] = useState<Member[]>([]);
  const [membersLoading, setMembersLoading] = useState(true);

  const canInvite = user?.role === "owner" || user?.role === "admin";

  useEffect(() => {
    apiClient
      .get<Member[]>("/v1/members")
      .then((res) => setMembers(res.data))
      .catch(() => undefined)
      .finally(() => setMembersLoading(false));
  }, []);

  async function handleInvite(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setInviteUrl(null);
    setCopied(false);
    setSubmitting(true);
    try {
      const res = await orgApi.invite({ email, role });
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

  async function handleCopy() {
    if (!inviteUrl) return;
    try {
      await navigator.clipboard.writeText(inviteUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback: select the text manually
    }
  }

  return (
    <div className="max-w-2xl space-y-8">
      <div>
        <h2 className="text-xl font-semibold">Team</h2>
        <p className="mt-1 text-sm text-slate-400">
          Manage members and send invitations.
        </p>
      </div>

      {/* Member list */}
      <div className="card">
        <h3 className="mb-4 text-sm font-semibold text-slate-200">Members</h3>
        {membersLoading ? (
          <p className="text-xs text-slate-500">Loading…</p>
        ) : members.length === 0 ? (
          <p className="text-xs text-slate-500">No members found.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-slate-500">
                <th className="pb-2 pr-4 font-medium">Email</th>
                <th className="pb-2 pr-4 font-medium">Role</th>
                <th className="pb-2 font-medium">Last login</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-border">
              {members.map((m) => (
                <tr key={m.user_id}>
                  <td className="py-2 pr-4 text-slate-200">{m.email}</td>
                  <td className="py-2 pr-4">
                    <span className="rounded bg-brand-900/60 px-1.5 py-0.5 text-xs text-brand-300">
                      {m.role}
                    </span>
                  </td>
                  <td className="py-2 text-xs text-slate-500">
                    {m.last_login_at
                      ? new Date(m.last_login_at).toLocaleDateString()
                      : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Invite form */}
      {canInvite && (
        <div className="card">
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
                <div className="flex items-center gap-2">
                  <p className="flex-1 break-all font-mono text-xs text-brand-200">
                    {inviteUrl}
                  </p>
                  <button
                    type="button"
                    onClick={handleCopy}
                    className="shrink-0 rounded px-2 py-1 text-xs font-medium bg-brand-700 text-brand-100 hover:bg-brand-600 transition-colors"
                  >
                    {copied ? "Copied!" : "Copy"}
                  </button>
                </div>
              </div>
            )}

            <button type="submit" disabled={submitting} className="btn-primary">
              {submitting ? "Sending…" : "Send invitation"}
            </button>
          </form>
        </div>
      )}

      {!canInvite && (
        <div className="card">
          <p className="text-sm text-slate-400">
            Only owners and admins can invite new members.
          </p>
        </div>
      )}
    </div>
  );
}

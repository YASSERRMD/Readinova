import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

const SECTORS = [
  "finance",
  "healthcare",
  "mfg",
  "retail",
  "tech",
  "government",
  "education",
  "other",
];
const SIZE_BANDS = ["1-50", "51-250", "251-1000", "1001+"];

export function SignupPage() {
  const { signup } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [form, setForm] = useState({
    org_name: "",
    org_slug: "",
    country_code: "GB",
    sector: "finance",
    size_band: "1-50",
    email: "",
    password: "",
  });

  function set(field: string, value: string) {
    setForm((f) => ({ ...f, [field]: value }));
  }

  function deriveSlug(name: string) {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signup(form);
      navigate("/app/assessments");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data
          ?.error ?? "Signup failed. Please try again.";
      setError(msg);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4 py-12">
      <div className="w-full max-w-md">
        <p className="mb-2 text-xs font-bold uppercase tracking-[0.2em] text-brand-400">
          Readinova
        </p>
        <h1 className="mb-8 text-2xl font-semibold">
          Create your organisation
        </h1>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="label mb-1" htmlFor="org_name">
              Organisation name
            </label>
            <input
              id="org_name"
              type="text"
              required
              className="input"
              value={form.org_name}
              onChange={(e) => {
                set("org_name", e.target.value);
                set("org_slug", deriveSlug(e.target.value));
              }}
            />
          </div>

          <div>
            <label className="label mb-1" htmlFor="org_slug">
              Slug
            </label>
            <input
              id="org_slug"
              type="text"
              required
              pattern="[a-z0-9-]+"
              className="input font-mono text-xs"
              value={form.org_slug}
              onChange={(e) => set("org_slug", e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="label mb-1" htmlFor="sector">
                Sector
              </label>
              <select
                id="sector"
                className="input"
                value={form.sector}
                onChange={(e) => set("sector", e.target.value)}
              >
                {SECTORS.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="label mb-1" htmlFor="size_band">
                Size
              </label>
              <select
                id="size_band"
                className="input"
                value={form.size_band}
                onChange={(e) => set("size_band", e.target.value)}
              >
                {SIZE_BANDS.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="label mb-1" htmlFor="country_code">
              Country code
            </label>
            <input
              id="country_code"
              type="text"
              maxLength={2}
              required
              className="input uppercase"
              value={form.country_code}
              onChange={(e) =>
                set("country_code", e.target.value.toUpperCase())
              }
            />
          </div>

          <hr className="border-surface-border" />

          <div>
            <label className="label mb-1" htmlFor="email">
              Your email
            </label>
            <input
              id="email"
              type="email"
              required
              autoComplete="email"
              className="input"
              value={form.email}
              onChange={(e) => set("email", e.target.value)}
            />
          </div>

          <div>
            <label className="label mb-1" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              required
              minLength={8}
              autoComplete="new-password"
              className="input"
              value={form.password}
              onChange={(e) => set("password", e.target.value)}
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
            {submitting ? "Creating…" : "Create organisation"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-slate-500">
          Already have an account?{" "}
          <Link to="/login" className="text-brand-400 hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}

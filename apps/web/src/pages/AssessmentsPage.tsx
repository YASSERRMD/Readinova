import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import {
  assessmentsApi,
  type AssessmentSummary,
  type Framework,
} from "../api/assessments";
import { extractApiError } from "../api/errors";

const STATUS_BADGE: Record<string, string> = {
  draft: "bg-slate-700 text-slate-300",
  in_progress: "bg-blue-900 text-blue-300",
  ready_to_score: "bg-yellow-900 text-yellow-300",
  scored: "bg-green-900 text-green-300",
  archived: "bg-slate-800 text-slate-500",
};

export function AssessmentsPage() {
  const [assessments, setAssessments] = useState<AssessmentSummary[]>([]);
  const [frameworks, setFrameworks] = useState<Framework[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Create modal state.
  const [showCreate, setShowCreate] = useState(false);
  const [createTitle, setCreateTitle] = useState("");
  const [createFrameworkId, setCreateFrameworkId] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    Promise.all([assessmentsApi.list(), assessmentsApi.listFrameworks()])
      .then(([a, f]) => {
        setAssessments(a.data);
        setFrameworks(f.data);
        if (f.data.length > 0) setCreateFrameworkId(f.data[0].id);
      })
      .catch(() => setError("Failed to load assessments."))
      .finally(() => setLoading(false));
  }, []);

  function openCreate() {
    setCreateTitle("");
    setCreateError(null);
    setShowCreate(true);
    requestAnimationFrame(() => dialogRef.current?.showModal());
  }

  function closeCreate() {
    setShowCreate(false);
    dialogRef.current?.close();
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreateError(null);
    setCreating(true);
    try {
      const res = await assessmentsApi.create(createFrameworkId, createTitle);
      const newAssessment: AssessmentSummary = {
        id: res.data.id,
        title: createTitle,
        status: res.data.status,
        framework_id: createFrameworkId,
        created_at: new Date().toISOString(),
      };
      setAssessments((prev) => [newAssessment, ...prev]);
      closeCreate();
    } catch (err: unknown) {
      setCreateError(extractApiError(err, "Failed to create assessment."));
    } finally {
      setCreating(false);
    }
  }

  if (loading) {
    return <p className="text-sm text-slate-400">Loading…</p>;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h2 className="text-xl font-semibold">Assessments</h2>
        <button onClick={openCreate} className="btn-primary text-sm">
          + New assessment
        </button>
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {assessments.length === 0 && !error && (
        <p className="text-sm text-slate-400">
          No assessments yet.{" "}
          <button
            onClick={openCreate}
            className="text-brand-400 hover:underline"
          >
            Create one now.
          </button>
        </p>
      )}

      <div className="space-y-3">
        {assessments.map((a) => (
          <div
            key={a.id}
            className="card flex items-center justify-between gap-4"
          >
            <div>
              <p className="font-medium text-slate-100">{a.title}</p>
              <p className="mt-0.5 text-xs text-slate-500">
                {new Date(a.created_at).toLocaleDateString()}
              </p>
            </div>
            <div className="flex items-center gap-3">
              <span
                className={`rounded px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[a.status] ?? STATUS_BADGE.draft}`}
              >
                {a.status.replace(/_/g, " ")}
              </span>
              {a.status === "in_progress" && (
                <Link
                  to={`/app/assessments/${a.id}/questionnaire`}
                  className="btn-primary text-xs px-3 py-1"
                >
                  Continue
                </Link>
              )}
              {a.status === "scored" && (
                <Link
                  to={`/app/assessments/${a.id}/dashboard`}
                  className="btn-primary text-xs px-3 py-1"
                >
                  View Results
                </Link>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Create Assessment modal */}
      {showCreate && (
        <dialog
          ref={dialogRef}
          className="rounded-xl bg-surface p-6 shadow-xl backdrop:bg-black/60 w-full max-w-sm"
          onClose={closeCreate}
        >
          <h3 className="mb-4 text-lg font-semibold">New assessment</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="label mb-1" htmlFor="c-title">
                Title
              </label>
              <input
                id="c-title"
                type="text"
                required
                className="input"
                placeholder="Q3 AI Readiness Review"
                value={createTitle}
                onChange={(e) => setCreateTitle(e.target.value)}
              />
            </div>

            {frameworks.length > 0 && (
              <div>
                <label className="label mb-1" htmlFor="c-framework">
                  Framework
                </label>
                <select
                  id="c-framework"
                  className="input"
                  value={createFrameworkId}
                  onChange={(e) => setCreateFrameworkId(e.target.value)}
                >
                  {frameworks.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.name} v{f.version}
                    </option>
                  ))}
                </select>
              </div>
            )}

            {createError && (
              <p className="rounded-md bg-red-900/40 px-3 py-2 text-sm text-red-400">
                {createError}
              </p>
            )}

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={creating}
                className="btn-primary flex-1"
              >
                {creating ? "Creating…" : "Create"}
              </button>
              <button
                type="button"
                onClick={closeCreate}
                className="btn-ghost flex-1"
              >
                Cancel
              </button>
            </div>
          </form>
        </dialog>
      )}
    </div>
  );
}

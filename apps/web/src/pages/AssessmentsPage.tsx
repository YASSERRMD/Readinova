import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { assessmentsApi, type AssessmentSummary } from "../api/assessments";

const STATUS_BADGE: Record<string, string> = {
  draft: "bg-slate-700 text-slate-300",
  in_progress: "bg-blue-900 text-blue-300",
  ready_to_score: "bg-yellow-900 text-yellow-300",
  scored: "bg-green-900 text-green-300",
  archived: "bg-slate-800 text-slate-500",
};

export function AssessmentsPage() {
  const [assessments, setAssessments] = useState<AssessmentSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    assessmentsApi
      .list()
      .then((res) => setAssessments(res.data))
      .catch(() => setError("Failed to load assessments."))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <p className="text-sm text-slate-400">Loading…</p>;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h2 className="text-xl font-semibold">Assessments</h2>
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {assessments.length === 0 && !error && (
        <p className="text-sm text-slate-400">
          No assessments yet. Ask your admin to create one.
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
    </div>
  );
}

import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { scoringApi, type ScoringResult, type PerceptionGapResult } from "../api/scoring";
import { ReadinessRadarChart } from "../components/RadarChart";
import { DimensionCard } from "../components/DimensionCard";

export function DashboardPage() {
  const { id: assessmentId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [result, setResult] = useState<ScoringResult | null>(null);
  const [gapResult, setGapResult] = useState<PerceptionGapResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [gapLoading, setGapLoading] = useState(false);

  useEffect(() => {
    if (!assessmentId) return;
    scoringApi
      .getResult(assessmentId)
      .then((res) => setResult(res.data))
      .catch(() => setError("No scoring result available for this assessment."))
      .finally(() => setLoading(false));
    scoringApi.getPerceptionGap(assessmentId).then((res) => setGapResult(res.data)).catch(() => {});
  }, [assessmentId]);

  function runGap() {
    if (!assessmentId) return;
    setGapLoading(true);
    scoringApi.runPerceptionGap(assessmentId)
      .then((res) => setGapResult(res.data))
      .finally(() => setGapLoading(false));
  }

  if (loading) {
    return <p className="text-sm text-slate-400">Loading results…</p>;
  }

  if (error || !result) {
    return (
      <div>
        <button
          onClick={() => navigate("/app/assessments")}
          className="btn-ghost text-xs mb-6"
        >
          ← Back to assessments
        </button>
        <p className="text-sm text-red-400">{error ?? "No result"}</p>
      </div>
    );
  }

  const composite = Math.round(result.composite_layer_a);
  const apiBase = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
  const reportUrl = `${apiBase}/v1/assessments/${assessmentId}/report?format=pdf`;

  function compositeColor(score: number): string {
    if (score >= 75) return "text-green-400";
    if (score >= 50) return "text-yellow-400";
    if (score >= 25) return "text-orange-400";
    return "text-red-400";
  }

  return (
    <div className="mx-auto max-w-5xl space-y-10">
      <div className="flex items-start justify-between gap-4">
        <button
          onClick={() => navigate("/app/assessments")}
          className="btn-ghost text-xs"
        >
          ← Back
        </button>
        <div className="flex items-center gap-4">
          <span className="text-xs text-slate-500">
            Engine: {result.engine_version} · Framework:{" "}
            {result.framework_version}
          </span>
          <a
            href={reportUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="btn-primary text-xs px-3 py-1.5"
          >
            Download PDF
          </a>
        </div>
      </div>

      {/* Composite score hero */}
      <div className="text-center">
        <p className="text-xs font-bold uppercase tracking-[0.2em] text-slate-500">
          AI Readiness Composite Score
        </p>
        <p
          className={`mt-2 text-7xl font-bold tabular-nums ${compositeColor(composite)}`}
        >
          {composite}
        </p>
        <p className="mt-1 text-sm text-slate-500">out of 100</p>
      </div>

      {/* Binding constraint callout */}
      {result.binding_constraint_dimension && (
        <div className="rounded-lg border border-orange-700/50 bg-orange-950/30 p-4">
          <p className="text-xs font-bold uppercase tracking-wide text-orange-400">
            Binding constraint
          </p>
          <p className="mt-1 text-sm text-orange-200">
            <span className="font-semibold">
              {result.binding_constraint_dimension
                .replace(/_/g, " ")
                .replace(/\b\w/g, (c) => c.toUpperCase())}
            </span>{" "}
            is limiting your composite score with a dimension score of{" "}
            <span className="font-semibold">
              {Math.round(result.binding_constraint_score)}
            </span>
            /100.
          </p>
        </div>
      )}

      {/* Radar chart + derived indices */}
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
        <div className="card">
          <h3 className="mb-4 text-sm font-semibold text-slate-300">
            Dimension radar
          </h3>
          <ReadinessRadarChart dimensionScores={result.dimension_scores} />
        </div>

        <div className="space-y-4">
          <h3 className="text-sm font-semibold text-slate-300">
            Derived indices
          </h3>
          {[
            { key: "readiness_index", label: "Readiness Index" },
            { key: "governance_risk_score", label: "Governance & Risk" },
            { key: "execution_capacity_score", label: "Execution Capacity" },
            { key: "value_realisation_score", label: "Value Realisation" },
          ].map(({ key, label }) => {
            const score = Math.round(
              result.derived[key as keyof typeof result.derived] as number,
            );
            return (
              <div key={key} className="card py-3">
                <div className="flex items-center justify-between">
                  <p className="text-sm text-slate-300">{label}</p>
                  <p className="text-base font-bold tabular-nums text-brand-300">
                    {score}
                  </p>
                </div>
                <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-surface-border">
                  <div
                    className="h-full rounded-full bg-brand-500"
                    style={{ width: `${score}%` }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Perception Gap */}
      <div className="card space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-300">Perception Gap Analysis</h3>
          <button
            className="btn-primary text-xs px-3 py-1.5"
            disabled={gapLoading}
            onClick={runGap}
          >
            {gapLoading ? "Computing…" : "Run Perception Gap"}
          </button>
        </div>
        {gapResult ? (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            {[
              { label: "Self-Assessment (S)", value: gapResult.layer_a_score, color: "text-brand-300" },
              { label: "Evidence Score (E)", value: gapResult.layer_b_score, color: "text-green-400" },
              { label: "Perception Gap (S−E)", value: gapResult.gap_score, color: Math.abs(gapResult.gap_score) > 20 ? "text-red-400" : "text-yellow-400" },
              { label: "Master Composite", value: gapResult.master_composite, color: "text-white" },
            ].map(({ label, value, color }) => (
              <div key={label} className="rounded-lg bg-surface p-3 text-center">
                <p className="text-xs text-slate-500">{label}</p>
                <p className={`mt-1 text-2xl font-bold tabular-nums ${color}`}>{value}</p>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-xs text-slate-500">
            No perception gap data yet. Sync evidence connectors first, then run the analysis.
          </p>
        )}
      </div>

      {/* Dimension cards */}
      <div>
        <h3 className="mb-4 text-sm font-semibold text-slate-300">
          Dimension scores
        </h3>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Object.entries(result.dimension_scores).map(([slug, score]) => (
            <DimensionCard
              key={slug}
              slug={slug}
              score={score}
              isBinding={slug === result.binding_constraint_dimension}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

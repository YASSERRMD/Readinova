import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { recommendApi, type Recommendation } from '../api/recommendations';

const WAVE_LABELS: Record<number, { label: string; description: string; color: string }> = {
  1: { label: 'Wave 1 — Act Now', description: 'High-priority items with maximum impact on your readiness score.', color: 'border-red-700/50 bg-red-950/20' },
  2: { label: 'Wave 2 — Next Quarter', description: 'Important improvements to plan for the coming months.', color: 'border-yellow-700/50 bg-yellow-950/20' },
  3: { label: 'Wave 3 — Roadmap', description: 'Longer-term investments that will sustain your AI programme.', color: 'border-surface-700 bg-surface-800/40' },
};

const EFFORT_COLORS: Record<string, string> = {
  low: 'text-green-400 bg-green-950/50',
  medium: 'text-yellow-400 bg-yellow-950/50',
  high: 'text-red-400 bg-red-950/50',
};

const IMPACT_COLORS: Record<string, string> = {
  low: 'text-slate-400 bg-surface-700',
  medium: 'text-brand-300 bg-brand-950/50',
  high: 'text-white bg-brand-800/50',
};

function RecommendationCard({ rec }: { rec: Recommendation }) {
  return (
    <div className="card space-y-2">
      <div className="flex items-start justify-between gap-2">
        <p className="text-sm font-semibold text-white">{rec.title}</p>
        <div className="flex shrink-0 gap-1.5">
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${IMPACT_COLORS[rec.impact]}`}>
            {rec.impact} impact
          </span>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${EFFORT_COLORS[rec.effort]}`}>
            {rec.effort} effort
          </span>
        </div>
      </div>
      <p className="text-xs text-slate-400">{rec.description}</p>
      <div className="flex items-center justify-between">
        <span className="text-xs text-slate-600">
          {rec.dimension_slug.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())}
        </span>
        <span className="text-xs text-slate-600">Priority: {rec.priority}</span>
      </div>
    </div>
  );
}

export default function RecommendationsPage() {
  const { id: assessmentId } = useParams<{ id: string }>();

  const { data, isLoading, error } = useQuery({
    queryKey: ['recommendations', assessmentId],
    queryFn: () => recommendApi.get(assessmentId!),
    enabled: Boolean(assessmentId),
    retry: false,
  });

  if (isLoading) {
    return <div className="p-8 text-surface-400">Loading recommendations…</div>;
  }

  if (error || !data) {
    return (
      <div className="p-8 space-y-4">
        <Link to="/app/assessments" className="btn-ghost text-xs">← Back to assessments</Link>
        <p className="text-sm text-red-400">
          No recommendations available. Complete scoring first and ensure you have a Starter or higher subscription.
        </p>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-4xl mx-auto space-y-8">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Link to={`/app/assessments/${assessmentId}/dashboard`} className="btn-ghost text-xs">
            ← Back to dashboard
          </Link>
          <h1 className="mt-4 text-2xl font-bold text-white">Recommendations</h1>
          <p className="text-surface-400 mt-1">{data.total} recommendations across 3 waves</p>
        </div>
      </div>

      {([1, 2, 3] as const).map(wave => {
        const waveKey = `wave_${wave}` as keyof typeof data.waves;
        const recs = data.waves[waveKey] ?? [];
        if (recs.length === 0) return null;
        const meta = WAVE_LABELS[wave];

        return (
          <div key={wave} className={`rounded-xl border p-6 space-y-4 ${meta.color}`}>
            <div>
              <h2 className="text-base font-semibold text-white">{meta.label}</h2>
              <p className="text-xs text-slate-400 mt-0.5">{meta.description}</p>
            </div>
            <div className="space-y-3">
              {recs.map(rec => <RecommendationCard key={rec.id} rec={rec} />)}
            </div>
          </div>
        );
      })}
    </div>
  );
}

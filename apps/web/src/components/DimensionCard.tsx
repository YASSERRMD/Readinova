interface Props {
  slug: string;
  score: number;
  isBinding?: boolean;
}

function scoreColor(score: number): string {
  if (score >= 75) return "text-green-400";
  if (score >= 50) return "text-yellow-400";
  if (score >= 25) return "text-orange-400";
  return "text-red-400";
}

function scoreBarColor(score: number): string {
  if (score >= 75) return "bg-green-500";
  if (score >= 50) return "bg-yellow-500";
  if (score >= 25) return "bg-orange-500";
  return "bg-red-500";
}

export function DimensionCard({ slug, score, isBinding }: Props) {
  const label = slug.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  const rounded = Math.round(score);

  return (
    <div
      className={`card relative overflow-hidden ${
        isBinding ? "ring-1 ring-orange-500/60" : ""
      }`}
    >
      {isBinding && (
        <span className="absolute right-3 top-3 rounded bg-orange-900/60 px-1.5 py-0.5 text-xs font-medium text-orange-300">
          Binding constraint
        </span>
      )}
      <p className="text-sm font-medium text-slate-300">{label}</p>
      <p className={`mt-1 text-3xl font-bold tabular-nums ${scoreColor(rounded)}`}>
        {rounded}
        <span className="ml-0.5 text-base font-normal text-slate-500">/100</span>
      </p>
      <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-surface-border">
        <div
          className={`h-full rounded-full transition-all ${scoreBarColor(rounded)}`}
          style={{ width: `${rounded}%` }}
        />
      </div>
    </div>
  );
}

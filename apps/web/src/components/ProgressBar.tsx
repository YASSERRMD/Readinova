interface Props {
  total: number;
  answered: number;
  label?: string;
}

export function ProgressBar({ total, answered, label }: Props) {
  const pct = total === 0 ? 100 : Math.round((answered / total) * 100);

  return (
    <div>
      {label && (
        <div className="mb-1 flex justify-between text-xs text-slate-400">
          <span>{label}</span>
          <span>
            {answered}/{total}
          </span>
        </div>
      )}
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-border">
        <div
          className="h-full rounded-full bg-brand-500 transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

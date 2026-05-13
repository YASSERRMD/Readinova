import type { RubricLevel } from "../api/assessments";

interface Props {
  rubricLevels: RubricLevel[];
  selected: number | null;
  onChange: (level: number) => void;
  disabled?: boolean;
}

const LEVEL_COLORS = [
  "border-red-700 data-[active=true]:bg-red-900/50 data-[active=true]:border-red-500",
  "border-orange-700 data-[active=true]:bg-orange-900/50 data-[active=true]:border-orange-500",
  "border-yellow-700 data-[active=true]:bg-yellow-900/50 data-[active=true]:border-yellow-500",
  "border-blue-700 data-[active=true]:bg-blue-900/50 data-[active=true]:border-blue-500",
  "border-green-700 data-[active=true]:bg-green-900/50 data-[active=true]:border-green-500",
];

export function RubricCard({
  rubricLevels,
  selected,
  onChange,
  disabled,
}: Props) {
  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-5">
      {rubricLevels.map((rl) => {
        const isActive = selected === rl.level;
        const colorClass = LEVEL_COLORS[rl.level] ?? LEVEL_COLORS[0];
        return (
          <button
            key={rl.level}
            type="button"
            disabled={disabled}
            data-active={isActive}
            onClick={() => onChange(rl.level)}
            className={`rounded-lg border p-3 text-left transition-colors hover:bg-surface-muted/80 disabled:cursor-not-allowed disabled:opacity-50 ${colorClass}`}
          >
            <span className="block text-xs font-bold tabular-nums text-slate-400">
              L{rl.level}
            </span>
            <span className="mt-1 block text-sm font-semibold text-slate-100">
              {rl.label}
            </span>
            <span className="mt-1 block text-xs leading-relaxed text-slate-400 line-clamp-3">
              {rl.description}
            </span>
          </button>
        );
      })}
    </div>
  );
}

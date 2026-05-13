import {
  Radar,
  RadarChart,
  PolarGrid,
  PolarAngleAxis,
  PolarRadiusAxis,
  ResponsiveContainer,
  Tooltip,
} from "recharts";

interface Props {
  dimensionScores: Record<string, number>;
}

export function ReadinessRadarChart({ dimensionScores }: Props) {
  const data = Object.entries(dimensionScores).map(([dim, score]) => ({
    subject: dim.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
    score: Math.round(score),
    fullMark: 100,
  }));

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center text-xs text-slate-500">
        No dimension scores
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={320}>
      <RadarChart cx="50%" cy="50%" outerRadius="75%" data={data}>
        <PolarGrid stroke="#334155" />
        <PolarAngleAxis
          dataKey="subject"
          tick={{ fill: "#94a3b8", fontSize: 11 }}
        />
        <PolarRadiusAxis
          angle={90}
          domain={[0, 100]}
          tick={{ fill: "#64748b", fontSize: 10 }}
        />
        <Radar
          name="Score"
          dataKey="score"
          stroke="#06b6d4"
          fill="#06b6d4"
          fillOpacity={0.25}
          strokeWidth={2}
        />
        <Tooltip
          contentStyle={{
            background: "#1e293b",
            border: "1px solid #334155",
            borderRadius: "6px",
            fontSize: "12px",
          }}
          formatter={(value: number) => [`${value}`, "Score"]}
        />
      </RadarChart>
    </ResponsiveContainer>
  );
}

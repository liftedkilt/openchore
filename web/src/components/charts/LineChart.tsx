import React from 'react';

export interface LineChartPoint {
  label: string;
  value: number;
}

interface LineChartSeries {
  data: LineChartPoint[];
  color?: string;
  label?: string;
  filled?: boolean;
}

interface LineChartProps {
  series: LineChartSeries[];
  height?: number;
}

export const LineChart: React.FC<LineChartProps> = ({
  series,
  height = 200,
}) => {
  if (series.length === 0 || series.every(s => s.data.length === 0)) {
    return (
      <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
        No data
      </div>
    );
  }

  const allValues = series.flatMap(s => s.data.map(d => d.value));
  const maxValue = Math.max(...allValues, 1);
  const maxLen = Math.max(...series.map(s => s.data.length));
  const labels = series[0]?.data.map(d => d.label) || [];

  const padding = { top: 10, right: 15, bottom: 40, left: 35 };
  const chartWidth = 400;
  const chartHeight = height;
  const innerWidth = chartWidth - padding.left - padding.right;
  const innerHeight = chartHeight - padding.top - padding.bottom;

  const getX = (i: number) => padding.left + (maxLen > 1 ? (i / (maxLen - 1)) * innerWidth : innerWidth / 2);
  const getY = (v: number) => padding.top + innerHeight - (v / maxValue) * innerHeight;

  // Show at most ~8 labels
  const labelStep = Math.max(1, Math.ceil(maxLen / 8));

  return (
    <svg width="100%" height={chartHeight} viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMidYMid meet">
      {/* Grid lines */}
      {[0, 0.25, 0.5, 0.75, 1].map((frac) => {
        const y = padding.top + innerHeight * (1 - frac);
        const val = Math.round(maxValue * frac);
        return (
          <g key={frac}>
            <line
              x1={padding.left}
              y1={y}
              x2={chartWidth - padding.right}
              y2={y}
              stroke="var(--text-secondary)"
              strokeOpacity={0.1}
              strokeDasharray="3,3"
            />
            <text
              x={padding.left - 6}
              y={y + 3}
              textAnchor="end"
              fill="var(--text-secondary)"
              fontSize="9"
            >
              {val}
            </text>
          </g>
        );
      })}

      {/* Series */}
      {series.map((s, si) => {
        const color = s.color || 'var(--accent-blue)';
        const points = s.data.map((d, i) => `${getX(i)},${getY(d.value)}`);

        if (points.length === 0) return null;

        const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p}`).join(' ');

        // Area fill
        const areaD = s.filled !== false
          ? `${pathD} L ${getX(s.data.length - 1)},${padding.top + innerHeight} L ${getX(0)},${padding.top + innerHeight} Z`
          : '';

        return (
          <g key={si}>
            {s.filled !== false && (
              <path d={areaD} fill={color} opacity={0.1} />
            )}
            <path d={pathD} fill="none" stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
            {s.data.map((d, i) => (
              <circle key={i} cx={getX(i)} cy={getY(d.value)} r={3} fill={color} />
            ))}
          </g>
        );
      })}

      {/* X-axis labels */}
      {labels.map((label, i) => {
        if (i % labelStep !== 0 && i !== labels.length - 1) return null;
        // Format: show just month/day for dates
        const short = label.length === 10 ? label.slice(5) : label;
        return (
          <text
            key={i}
            x={getX(i)}
            y={chartHeight - padding.bottom + 16}
            textAnchor="middle"
            fill="var(--text-secondary)"
            fontSize="9"
          >
            {short}
          </text>
        );
      })}

      {/* Legend */}
      {series.length > 1 && series.map((s, si) => {
        const color = s.color || 'var(--accent-blue)';
        return (
          <g key={si} transform={`translate(${padding.left + si * 90}, ${chartHeight - 6})`}>
            <rect width={10} height={3} rx={1.5} fill={color} />
            <text x={14} y={3} fill="var(--text-secondary)" fontSize="9">{s.label || `Series ${si + 1}`}</text>
          </g>
        );
      })}
    </svg>
  );
};

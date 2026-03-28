import React from 'react';

export interface BarChartItem {
  label: string;
  value: number;
  color?: string;
}

interface BarChartProps {
  data: BarChartItem[];
  height?: number;
  barColor?: string;
  showValues?: boolean;
  horizontal?: boolean;
}

export const BarChart: React.FC<BarChartProps> = ({
  data,
  height = 200,
  barColor = 'var(--accent-blue)',
  showValues = true,
  horizontal = false,
}) => {
  if (data.length === 0) {
    return (
      <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
        No data
      </div>
    );
  }

  const maxValue = Math.max(...data.map(d => d.value), 1);

  if (horizontal) {
    const barHeight = 28;
    const gap = 8;
    const labelWidth = 100;
    const svgHeight = data.length * (barHeight + gap);

    return (
      <svg width="100%" height={svgHeight} viewBox={`0 0 400 ${svgHeight}`} preserveAspectRatio="xMinYMin meet">
        {data.map((item, i) => {
          const y = i * (barHeight + gap);
          const barWidth = (item.value / maxValue) * 260;
          const color = item.color || barColor;
          return (
            <g key={i}>
              <text
                x={labelWidth - 8}
                y={y + barHeight / 2 + 1}
                textAnchor="end"
                fill="var(--text-secondary)"
                fontSize="11"
                dominantBaseline="middle"
              >
                {item.label.length > 14 ? item.label.slice(0, 13) + '...' : item.label}
              </text>
              <rect
                x={labelWidth}
                y={y}
                width={barWidth}
                height={barHeight}
                rx={6}
                fill={color}
                opacity={0.85}
              />
              {showValues && (
                <text
                  x={labelWidth + barWidth + 6}
                  y={y + barHeight / 2 + 1}
                  fill="var(--text-secondary)"
                  fontSize="11"
                  dominantBaseline="middle"
                >
                  {item.value}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    );
  }

  // Vertical bar chart
  const padding = { top: 10, right: 10, bottom: 40, left: 10 };
  const chartWidth = 400;
  const chartHeight = height;
  const innerWidth = chartWidth - padding.left - padding.right;
  const innerHeight = chartHeight - padding.top - padding.bottom;
  const barWidth = Math.min(40, (innerWidth / data.length) * 0.7);
  const barGap = (innerWidth - barWidth * data.length) / (data.length + 1);

  return (
    <svg width="100%" height={chartHeight} viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMidYMid meet">
      {/* Grid lines */}
      {[0, 0.25, 0.5, 0.75, 1].map((frac) => {
        const y = padding.top + innerHeight * (1 - frac);
        return (
          <line
            key={frac}
            x1={padding.left}
            y1={y}
            x2={chartWidth - padding.right}
            y2={y}
            stroke="var(--text-secondary)"
            strokeOpacity={0.1}
            strokeDasharray="3,3"
          />
        );
      })}
      {data.map((item, i) => {
        const x = padding.left + barGap + i * (barWidth + barGap);
        const barH = (item.value / maxValue) * innerHeight;
        const y = padding.top + innerHeight - barH;
        const color = item.color || barColor;
        return (
          <g key={i}>
            <rect
              x={x}
              y={y}
              width={barWidth}
              height={barH}
              rx={4}
              fill={color}
              opacity={0.85}
            />
            {showValues && item.value > 0 && (
              <text
                x={x + barWidth / 2}
                y={y - 4}
                textAnchor="middle"
                fill="var(--text-secondary)"
                fontSize="10"
              >
                {item.value}
              </text>
            )}
            <text
              x={x + barWidth / 2}
              y={chartHeight - padding.bottom + 14}
              textAnchor="middle"
              fill="var(--text-secondary)"
              fontSize="10"
            >
              {item.label}
            </text>
          </g>
        );
      })}
    </svg>
  );
};

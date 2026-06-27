import type { MetricKey } from "../data/dashboard";

function samplePoints(points: Array<{ ts: number; value: number }>, maxPoints: number) {
  if (points.length <= maxPoints) {
    return points;
  }
  const sampled: Array<{ ts: number; value: number }> = [];
  const lastIndex = points.length - 1;
  const step = lastIndex / (maxPoints - 1);
  for (let index = 0; index < maxPoints; index += 1) {
    sampled.push(points[Math.min(lastIndex, Math.round(index * step))]);
  }
  return sampled;
}

export function MetricSparkline(props: {
  metricKey?: MetricKey;
  points: Array<{ ts: number; value: number }>;
}) {
  const points = samplePoints(props.points, 36);
  if (points.length < 2) {
    return <div className="sparkline-empty">等待更多样本</div>;
  }

  const width = 220;
  const height = 72;
  const pad = 8;
  const values = points.map((point) => point.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const lastIndex = points.length - 1;
  const linePoints = points
    .map((point, index) => {
      const x = pad + (index / lastIndex) * (width - pad * 2);
      const y = height - pad - ((point.value - min) / range) * (height - pad * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  const areaPoints = `${pad},${height - pad} ${linePoints} ${width - pad},${height - pad}`;

  return (
    <svg
      className={`sparkline ${props.metricKey ? `metric-${props.metricKey}` : ""}`}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label="metric trend"
    >
      <polygon className="sparkline-area" points={areaPoints} />
      <polyline className="sparkline-line" points={linePoints} />
    </svg>
  );
}

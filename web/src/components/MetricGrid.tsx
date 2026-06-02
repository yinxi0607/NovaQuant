export function MetricGrid({
  items,
}: {
  items: Array<{ label: string; value: string; tone?: "bull" | "bear" | "warn" | "neutral" }>;
}) {
  return (
    <div className="metric-grid">
      {items.map((item) => (
        <article key={item.label} className="metric-card">
          <p className="muted">{item.label}</p>
          <strong className={`tone-${item.tone ?? "neutral"}`}>{item.value}</strong>
        </article>
      ))}
    </div>
  );
}

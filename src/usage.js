/* Token and cost totals: { input, output, cached } tokens, plus cost in USD when a harness reports one. */

/* Adds `part` into `total` and returns it; cost stays absent until some part reports one. */
export function addUsage(total, part) {
  for (const key of ['input', 'output', 'cached']) total[key] = (total[key] || 0) + (part[key] || 0);
  if (typeof part.cost === 'number') total.cost = (total.cost || 0) + part.cost;
  return total;
}

/* " 1.2k in 300 out $0.0100" for a status line, or '' when nothing was reported. */
export function formatUsage(usage) {
  if (!usage || (!usage.input && !usage.output && typeof usage.cost !== 'number')) return '';
  const k = (n) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n || 0));
  const cost = typeof usage.cost === 'number' ? ` $${usage.cost.toFixed(4)}` : '';
  return ` ${k(usage.input)} in ${k(usage.output)} out${cost}`;
}

export function formatMoney(micros: number, currency = 'CNY') {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'CNY',
    minimumFractionDigits: 2,
    maximumFractionDigits: 6,
  }).format(micros / 1_000_000);
}

export function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value));
}

export function formatLatency(value?: number) {
  if (value === undefined || value === null) return '-';
  if (value < 1000) return `${value} ms`;
  return `${(value / 1000).toFixed(2)} s`;
}

export function canViewProjectUsage(
  isOwner: boolean,
  isProjectOwner: boolean,
  systemScopes: string[],
  projectScopes: string[]
) {
  return isOwner || isProjectOwner || systemScopes.includes('read_requests') || projectScopes.includes('read_requests');
}

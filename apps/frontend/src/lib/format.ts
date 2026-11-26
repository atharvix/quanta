export function formatCurrency(amount: number, currency: string = 'INR'): string {
  try {
    return new Intl.NumberFormat('en-IN', {
      style: 'currency',
      currency: currency.toUpperCase(),
      maximumFractionDigits: 2
    }).format(amount);
  } catch {
    return `${currency.toUpperCase()} ${formatNumber(amount)}`;
  }
}

export function formatNumber(value: number, maximumFractionDigits: number = 2): string {
  return new Intl.NumberFormat('en-IN', {
    maximumFractionDigits
  }).format(value);
}

export function formatPercent(ratio: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'percent',
    minimumFractionDigits: 1,
    maximumFractionDigits: 2
  }).format(ratio);
}

export function formatDate(dateStr: string): string {
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return new Intl.DateTimeFormat('en-IN', {
      dateStyle: 'medium',
      timeStyle: 'short'
    }).format(d);
  } catch {
    return dateStr;
  }
}


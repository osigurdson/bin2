export function formatCompactNumber(value: number): string {
  const safeValue = Number.isFinite(value) ? value : 0;
  return new Intl.NumberFormat('en-US', {
    notation: 'compact',
    maximumFractionDigits: safeValue >= 100 ? 0 : 1,
  }).format(safeValue);
}

export function formatUsageDecimal(value: string, digits = 2, minimumDigits = 0): string {
  const numeric = Number.parseFloat(value);
  if (!Number.isFinite(numeric)) {
    return '0';
  }

  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: minimumDigits,
    maximumFractionDigits: digits,
  }).format(numeric);
}

type StorageMonthDisplay = {
  value: string;
  unit: string;
};

export function formatStorageMonthDisplay(valueGiBMonths: string): StorageMonthDisplay {
  const numeric = Number.parseFloat(valueGiBMonths);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return {
      value: '0.000',
      unit: 'KiB-months',
    };
  }

  let scaledValue = numeric;
  let unit = 'GiB-months';

  if (scaledValue >= 1024) {
    scaledValue /= 1024;
    unit = 'TiB-months';
  } else if (scaledValue < 1 / 1024) {
    scaledValue *= 1024 * 1024;
    unit = 'KiB-months';
  } else if (scaledValue < 1) {
    scaledValue *= 1024;
    unit = 'MiB-months';
  }

  return {
    value: new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(scaledValue),
    unit,
  };
}

export function formatUsdAmount(valueUsd: string): string {
  const numeric = Number.parseFloat(valueUsd);
  if (!Number.isFinite(numeric)) {
    return '$0.00';
  }

  const absoluteValue = Math.abs(numeric);
  if (absoluteValue > 0 && absoluteValue < 0.0001) {
    return numeric < 0 ? '>-$0.0001' : '<$0.0001';
  }

  let minimumFractionDigits = 2;
  let maximumFractionDigits = 2;

  if (absoluteValue > 0 && absoluteValue < 0.01) {
    minimumFractionDigits = 4;
    maximumFractionDigits = 4;
  } else if (absoluteValue < 1) {
    maximumFractionDigits = 4;
  }

  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits,
    maximumFractionDigits,
  }).format(numeric);
}

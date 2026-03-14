'use client';

import { useGetCurrentMonthUsageSummary } from "@/api/usage/hooks";
import { formatCompactNumber, formatStorageMonthDisplay, formatUsdAmount } from "@/lib/formatCompactNumber";

export default function MonthlyUsagePanel() {
  const {
    data: usageSummary,
    isLoading,
    isError,
  } = useGetCurrentMonthUsageSummary();

  const monthLabel = new Intl.DateTimeFormat('en-US', {
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(new Date());

  const storageDisplay = formatStorageMonthDisplay(usageSummary?.storageGiBMonths ?? "0");
  const storageUnit = storageDisplay.unit.replace('-months', '·mo');
  const totalCharges = formatUsdAmount(usageSummary?.totalChargeUsd ?? "0");

  return (
    <section>
      <div className="mb-3 flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-widest opacity-40">Usage</span>
        <span className="text-xs opacity-40">{monthLabel} UTC</span>
      </div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <UsageCard label="Layer pulls" isLoading={isLoading} isError={isError}>
          <span className="text-2xl font-bold leading-tight">
            {formatCompactNumber(usageSummary?.pullOpCount ?? 0)}
          </span>
        </UsageCard>
        <UsageCard label="Layer pushes" isLoading={isLoading} isError={isError}>
          <span className="text-2xl font-bold leading-tight">
            {formatCompactNumber(usageSummary?.pushOpCount ?? 0)}
          </span>
        </UsageCard>
        <UsageCard label="Storage consumed" unit={storageUnit} isLoading={isLoading} isError={isError}>
          <span className="text-2xl font-bold leading-tight">{storageDisplay.value}</span>
        </UsageCard>
        <UsageCard label="Charges" unit="Month to date" isLoading={isLoading} isError={isError} green>
          <span className="text-2xl font-bold leading-tight">{totalCharges}</span>
        </UsageCard>
      </div>
    </section>
  );
}

type UsageCardProps = {
  label: string;
  unit?: string;
  isLoading: boolean;
  isError: boolean;
  green?: boolean;
  children: React.ReactNode;
};

function UsageCard({ label, unit, isLoading, isError, green, children }: UsageCardProps) {
  const base = "flex flex-col items-center justify-center gap-2 rounded-xl px-4 py-2.5";
  const colors = green
    ? "bg-success/10"
    : "bg-base-200/60";

  return (
    <div className={`${base} ${colors}`}>
      <div className="flex flex-col items-center gap-0.5">
        <span className="text-xs font-medium opacity-50">{label}</span>
        {unit && <span className="text-xs opacity-40">({unit})</span>}
      </div>
      <div className="flex items-baseline">
        {isLoading ? (
          <span className="text-sm opacity-40">—</span>
        ) : isError ? (
          <span className="text-sm text-error opacity-70">Unavailable</span>
        ) : (
          children
        )}
      </div>
    </div>
  );
}

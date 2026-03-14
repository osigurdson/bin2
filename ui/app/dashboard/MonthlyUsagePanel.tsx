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
  const totalCharges = formatUsdAmount(usageSummary?.totalChargeUsd ?? "0");

  return (
    <details
      className="collapse collapse-arrow rounded-2xl bg-base-100 mt-4"
      aria-label="Usage"
    >
      <summary className="collapse-title flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2 pr-10">
        <div className="text-xs font-bold">
          Usage
        </div>
        <div className="text-[0.8rem] opacity-70">{monthLabel} UTC</div>
      </summary>
      <div className="collapse-content pt-0">
        <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-4">
          <UsageSummaryCard
            label="Layer Pull"
            value={formatCompactNumber(usageSummary?.pullOpCount ?? 0)}
            meta="Count"
            isLoading={isLoading}
            isError={isError}
          />
          <UsageSummaryCard
            label="Layer Push"
            value={formatCompactNumber(usageSummary?.pushOpCount ?? 0)}
            meta="Count"
            isLoading={isLoading}
            isError={isError}
          />
          <UsageSummaryCard
            label="Storage"
            value={storageDisplay.value}
            meta={storageDisplay.unit}
            isLoading={isLoading}
            isError={isError}
          />
          <UsageSummaryCard
            label="Charges"
            value={totalCharges}
            meta="Month to date"
            isLoading={isLoading}
            isError={isError}
          />
        </div>
      </div>
    </details>
  );
}

type UsageSummaryCardProps = {
  label: string;
  value: string;
  meta: string;
  isLoading: boolean;
  isError: boolean;
};

function UsageSummaryCard(props: UsageSummaryCardProps) {
  let value = props.value;
  let metaClassName = "text-sm opacity-70";

  if (props.isLoading) {
    value = "Loading";
    metaClassName = "text-sm text-primary";
  } else if (props.isError) {
    value = "Unavailable";
    metaClassName = "text-sm text-error";
  }

  return (
    <div className="flex min-h-28 flex-col gap-1.5 rounded-lg border border-base-300 bg-base-200 px-4 py-4">
      <div className="text-sm font-bold uppercase tracking-[0.08em]">{props.label}</div>
      <div className="text-sm font-bold leading-none">{value}</div>
      <div className={metaClassName}>{props.meta}</div>
    </div>
  );
}

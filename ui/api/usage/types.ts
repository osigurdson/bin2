export type UsageSummary = {
  from: string;
  to: string;
  asOf: string;
  pushOpCount: number;
  pullOpCount: number;
  storageOpeningBytes: number;
  storageClosingBytes: number;
  storageByteSeconds: string;
  storageGiBHours: string;
  storageGiBMonths: string;
  storageChargeUsd: string;
  totalChargeUsd: string;
};

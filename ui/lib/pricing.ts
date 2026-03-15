export const pricing = {
  pushOpsPerMillion: 10,
  storagePerGiBMonth: 0.02,
  cdnPullOpsPerMillion: 2,
  freeCreditsPerMonth: 1.00,
  pushOpOverageMiBThreshold: 100,
} as const;

// Derived display strings
export const pricingDisplay = {
  pushOps:    `$${pricing.pushOpsPerMillion} per million operations`,
  storage:    `$${pricing.storagePerGiBMonth.toFixed(2)} per GiB-month`,
  cdnPulls:   `$${pricing.cdnPullOpsPerMillion} per million operations`,
  freeCredit: `$${pricing.freeCreditsPerMonth.toFixed(2)}`,
} as const;

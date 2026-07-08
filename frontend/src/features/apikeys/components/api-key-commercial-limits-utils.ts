import { FieldValues } from 'react-hook-form';

export const DEFAULT_COMMERCIAL_LIMIT_CURRENCY = 'CNY';

export type CommercialLimitsFormValues = FieldValues & {
  commercialLimits?: {
    enabled?: boolean;
    currency?: string | null;
    totalBudgetMicros?: number | null;
    dailyBudgetMicros?: number | null;
    monthlyBudgetMicros?: number | null;
    singleRequestMaxMicros?: number | null;
    notes?: string | null;
  } | null;
};

export function amountToMicros(value: string | number | null | undefined) {
  if (value === null || value === undefined || value === '') return null;

  const amount = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(amount) || amount < 0) return null;

  return Math.round(amount * 1_000_000);
}

export function microsToAmountInput(value?: number | null) {
  if (value === null || value === undefined) return '';
  return String(value / 1_000_000);
}

export function defaultCommercialLimits() {
  return {
    enabled: false,
    currency: DEFAULT_COMMERCIAL_LIMIT_CURRENCY,
    totalBudgetMicros: null,
    dailyBudgetMicros: null,
    monthlyBudgetMicros: null,
    singleRequestMaxMicros: null,
    notes: null,
  };
}

export function normalizeCommercialLimitsForSubmit(limits: CommercialLimitsFormValues['commercialLimits']) {
  const currency = (limits?.currency || DEFAULT_COMMERCIAL_LIMIT_CURRENCY).trim().toUpperCase();

  if (!limits?.enabled) {
    return {
      enabled: false,
      currency,
      totalBudgetMicros: null,
      dailyBudgetMicros: null,
      monthlyBudgetMicros: null,
      singleRequestMaxMicros: null,
      notes: null,
    };
  }

  return {
    enabled: true,
    currency,
    totalBudgetMicros: limits.totalBudgetMicros ?? null,
    dailyBudgetMicros: limits.dailyBudgetMicros ?? null,
    monthlyBudgetMicros: limits.monthlyBudgetMicros ?? null,
    singleRequestMaxMicros: limits.singleRequestMaxMicros ?? null,
    notes: limits.notes?.trim() || null,
  };
}

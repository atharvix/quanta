/**
 * Financial calculation engine for QUANTA.
 * Strictly data-driven, mathematically deterministic aggregations and allocations.
 * No synthetic multipliers, fabricated constants, or unverified optimization assumptions.
 */

import type { Asset, Liability, Treasury } from './types';

export interface PortfolioBalanceSheet {
  treasuryId: string;
  name: string;
  baseCurrency: string;
  totalAssets: number;
  totalLiabilities: number;
  netSurplus: number;
  minLiquidityRatio: number;
  riskTolerance: number;
  liabilitiesCount: number;
}

export interface BalanceSheetSummary {
  totalAssets: number;
  totalLiabilities: number;
  netSurplus: number;
  coverageRatio: number | null;
  portfolioCount: number;
  totalLiabilitiesCount: number;
  meanLiquidityFloor: number;
}

export interface AllocationResult {
  weights: number[];
  allocatedAmounts: number[];
  totalWeight: number;
  totalAllocated: number;
  mode: 'value-weighted' | 'equal-weight' | 'empty';
  label: string;
}

/**
 * Calculates total debt for a single treasury by aggregating actual persisted liability principal amounts.
 * Pure aggregation: empty lists return 0, deleted liabilities are omitted, invalid/negative values sanitized to 0.
 */
export function calculatePortfolioDebt(
  liabilities?: Array<Partial<Liability> | { principal?: number | string | null }> | null
): number {
  if (!liabilities || !Array.isArray(liabilities) || liabilities.length === 0) {
    return 0;
  }

  return liabilities.reduce((sum, item) => {
    if (!item) return sum;
    const raw = item.principal;
    if (raw === undefined || raw === null) return sum;
    const num = typeof raw === 'number' ? raw : parseFloat(String(raw));
    if (isNaN(num) || num < 0) return sum;
    return sum + num;
  }, 0);
}

/**
 * Aggregates consolidated balance sheet metrics across all registered treasuries.
 */
export function calculateBalanceSheet(
  treasuries: Array<Partial<Treasury> & { id: string }>,
  liabilitiesByTreasury: Record<string, Array<Partial<Liability>> | null | undefined>
): {
  portfolios: PortfolioBalanceSheet[];
  summary: BalanceSheetSummary;
} {
  if (!Array.isArray(treasuries) || treasuries.length === 0) {
    return {
      portfolios: [],
      summary: {
        totalAssets: 0,
        totalLiabilities: 0,
        netSurplus: 0,
        coverageRatio: null,
        portfolioCount: 0,
        totalLiabilitiesCount: 0,
        meanLiquidityFloor: 0
      }
    };
  }

  let totalAssets = 0;
  let totalLiabilities = 0;
  let totalLiabCount = 0;
  let liquiditySum = 0;

  const portfolios: PortfolioBalanceSheet[] = treasuries.map((t) => {
    const rawCapital = Number(t.total_capital);
    const assets = isNaN(rawCapital) || rawCapital < 0 ? 0 : rawCapital;
    totalAssets += assets;

    const tLiabilities = liabilitiesByTreasury[t.id] ?? [];
    const debt = calculatePortfolioDebt(tLiabilities);
    totalLiabilities += debt;
    totalLiabCount += Array.isArray(tLiabilities) ? tLiabilities.length : 0;

    const rawLiq = Number(t.min_liquidity_ratio);
    const minLiq = isNaN(rawLiq) || rawLiq < 0 ? 0 : rawLiq;
    liquiditySum += minLiq;

    const rawRisk = Number(t.risk_tolerance);
    const risk = isNaN(rawRisk) || rawRisk < 0 ? 0 : rawRisk;

    const surplus = assets - debt;

    return {
      treasuryId: t.id,
      name: t.name || 'Unnamed Portfolio',
      baseCurrency: t.base_currency || 'USD',
      totalAssets: assets,
      totalLiabilities: debt,
      netSurplus: surplus,
      minLiquidityRatio: minLiq,
      riskTolerance: risk,
      liabilitiesCount: Array.isArray(tLiabilities) ? tLiabilities.length : 0
    };
  });

  const netSurplus = totalAssets - totalLiabilities;
  const coverageRatio = totalLiabilities > 0 ? totalAssets / totalLiabilities : null;
  const meanLiquidityFloor = treasuries.length > 0 ? liquiditySum / treasuries.length : 0;

  return {
    portfolios,
    summary: {
      totalAssets,
      totalLiabilities,
      netSurplus,
      coverageRatio,
      portfolioCount: treasuries.length,
      totalLiabilitiesCount: totalLiabCount,
      meanLiquidityFloor
    }
  };
}

/**
 * Extracts explicit monetary value or principal holding from an asset record, if present.
 */
export function getAssetExplicitValue(
  asset: Partial<Asset> & {
    value?: number | string | null;
    amount?: number | string | null;
    principal?: number | string | null;
    metadata?: Record<string, any> | null;
  }
): number {
  if (!asset) return 0;

  // Direct numeric fields
  const candidates = [asset.value, asset.amount, asset.principal];
  for (const c of candidates) {
    if (c !== undefined && c !== null) {
      const num = typeof c === 'number' ? c : parseFloat(String(c));
      if (!isNaN(num) && num > 0) return num;
    }
  }

  // Metadata nested fields
  if (asset.metadata && typeof asset.metadata === 'object') {
    const metaCandidates = [
      asset.metadata.value,
      asset.metadata.amount,
      asset.metadata.principal,
      asset.metadata.holding_value
    ];
    for (const mc of metaCandidates) {
      if (mc !== undefined && mc !== null) {
        const num = typeof mc === 'number' ? mc : parseFloat(String(mc));
        if (!isNaN(num) && num > 0) return num;
      }
    }
  }

  return 0;
}

/**
 * Derives capital allocation weights and distributed capital amounts from the asset dataset.
 * If actual asset valuations are present, calculates value-weighted distribution (summing to 100%).
 * If explicit asset valuations are absent or zero, provides an equal-weight baseline (1/N) explicitly labeled as such.
 * Empty asset sets return zero weights and zero allocations.
 */
export function calculateCapitalAllocation(
  assets: Array<
    Partial<Asset> & {
      value?: number | string | null;
      amount?: number | string | null;
      principal?: number | string | null;
      metadata?: Record<string, any> | null;
    }
  >,
  totalCapital: number
): AllocationResult {
  const sanitizedCapital = Math.max(0, isNaN(totalCapital) ? 0 : totalCapital);

  if (!Array.isArray(assets) || assets.length === 0) {
    return {
      weights: [],
      allocatedAmounts: [],
      totalWeight: 0,
      totalAllocated: 0,
      mode: 'empty',
      label: 'No Assets Configured'
    };
  }

  const explicitValues = assets.map(getAssetExplicitValue);
  const totalExplicitValue = explicitValues.reduce((sum, v) => sum + v, 0);

  // Path A: Value-Weighted Distribution (actual asset valuation dataset exists)
  if (totalExplicitValue > 0) {
    const rawWeights = explicitValues.map((v) => v / totalExplicitValue);
    const sumRaw = rawWeights.reduce((s, w) => s + w, 0);
    // Normalize to prevent floating point inaccuracies
    const weights = sumRaw > 0 ? rawWeights.map((w) => w / sumRaw) : rawWeights;
    const allocatedAmounts = weights.map((w) => sanitizedCapital * w);

    return {
      weights,
      allocatedAmounts,
      totalWeight: weights.reduce((s, w) => s + w, 0),
      totalAllocated: allocatedAmounts.reduce((s, a) => s + a, 0),
      mode: 'value-weighted',
      label: 'Value-Weighted Distribution'
    };
  }

  // Path B: Equal-Weight Baseline (1/N unoptimized benchmark)
  const n = assets.length;
  const equalWeight = 1 / n;
  const weights = assets.map(() => equalWeight);
  const allocatedAmounts = weights.map((w) => sanitizedCapital * w);

  return {
    weights,
    allocatedAmounts,
    totalWeight: 1.0,
    totalAllocated: sanitizedCapital,
    mode: 'equal-weight',
    label: 'Equal-Weight Baseline (1/N)'
  };
}

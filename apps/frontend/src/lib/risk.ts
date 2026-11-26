import { formatNumber } from './format';
import type { BayesianEvidence } from './types';

/**
 * Parses raw allocations from strategy execution logs or AI engine output.
 * Handles JSON strings, object maps, array representations, and malformed inputs safely.
 */
export function parseAllocations(raw: any): Array<{ asset: string; amount: number; formattedAmount: string }> {
  if (!raw) return [];
  let obj = raw;
  if (typeof raw === 'string') {
    try {
      obj = JSON.parse(raw);
    } catch {
      return [{ asset: 'raw', amount: 0, formattedAmount: raw }];
    }
  }

  if (Array.isArray(obj)) {
    return obj.map((item, idx) => {
      if (typeof item === 'object' && item !== null) {
        const asset = item.asset || item.symbol || item.name || `Asset ${idx + 1}`;
        const amt = Number(item.amount ?? item.weight ?? item.allocation ?? 0);
        return {
          asset: String(asset),
          amount: isNaN(amt) ? 0 : amt,
          formattedAmount: isNaN(amt) ? '0' : formatNumber(amt)
        };
      }
      return { asset: `Item ${idx + 1}`, amount: 0, formattedAmount: String(item) };
    });
  }

  if (typeof obj === 'object') {
    return Object.entries(obj).map(([asset, amount]) => {
      const amt = Number(amount);
      return {
        asset,
        amount: isNaN(amt) ? 0 : amt,
        formattedAmount: isNaN(amt) ? String(amount) : formatNumber(amt)
      };
    });
  }

  return [{ asset: 'value', amount: 0, formattedAmount: String(obj) }];
}

/**
 * Extracts validated asset allocations for API payloads from either active or saved strategies.
 * Filters against the current asset catalog and excludes reserved artifact keys ('raw', 'value').
 */
export function extractAllocations(
  rawAllocations: any,
  currentAssets: Array<{ id: string }>
): Record<string, number> {
  if (!rawAllocations) return {};

  const parsed = parseAllocations(rawAllocations);
  const currentAssetIds = new Set(currentAssets.map((a) => a.id));
  const map: Record<string, number> = {};

  parsed.forEach((it) => {
    if (it.asset && it.asset !== 'raw' && it.asset !== 'value' && currentAssetIds.has(it.asset)) {
      map[it.asset] = it.amount;
    }
  });

  return map;
}

/**
 * Computes the minimum liquidity floor constraint threshold.
 * Accurately preserves an explicit 0.0 floor without accidental || fallback,
 * while respecting treasury defaults and fallbacks for missing/invalid input.
 */
export function calculateLiquidityFloor(
  minLiquidityInput: string | number | null | undefined,
  treasuryMinLiquidity?: number | null
): number {
  if (minLiquidityInput === null || minLiquidityInput === undefined || minLiquidityInput === '') {
    return treasuryMinLiquidity !== null && treasuryMinLiquidity !== undefined
      ? treasuryMinLiquidity
      : 0.20;
  }

  const parsed = typeof minLiquidityInput === 'number' ? minLiquidityInput : parseFloat(minLiquidityInput);
  if (isNaN(parsed)) {
    return treasuryMinLiquidity !== null && treasuryMinLiquidity !== undefined
      ? treasuryMinLiquidity
      : 0.20;
  }

  return parsed;
}

/**
 * Formats Bayesian conditioning evidence signals from UI controls.
 * Correctly distinguishes between unobserved (omitted), true, and false.
 * Returns undefined if all signals are unobserved so property is omitted in JSON serialization.
 */
export function formatBayesianEvidence(
  signalInflation: string,
  signalRateHike: string,
  signalRecession: string
): BayesianEvidence | undefined {
  const hasEvidence =
    signalInflation !== 'unobserved' ||
    signalRateHike !== 'unobserved' ||
    signalRecession !== 'unobserved';

  if (!hasEvidence) {
    return undefined;
  }

  const evidence: BayesianEvidence = {};
  if (signalInflation !== 'unobserved') evidence.inflation_high = signalInflation === 'true';
  if (signalRateHike !== 'unobserved') evidence.rate_hike = signalRateHike === 'true';
  if (signalRecession !== 'unobserved') evidence.recession_signal = signalRecession === 'true';

  return evidence;
}

/**
 * Maps risk tier labels from AI engine and backend to corresponding CSS badge classes.
 * Safely handles unknown, empty, or missing tiers.
 */
export function getRiskTierClass(tier: string | null | undefined): string {
  const t = (tier || '').toLowerCase().trim();
  if (t.includes('low') || t.includes('conservative')) {
    return 'badge-tier-low';
  }
  if (t.includes('mod')) {
    return 'badge-tier-moderate';
  }
  if (t.includes('high') || t.includes('crit') || t.includes('sever') || t.includes('aggress')) {
    return 'badge-tier-high';
  }
  return 'badge-tier-moderate';
}

/**
 * Formats Python action representation strings from the Alpha-Beta minimax game search
 * into human-readable financial descriptions, stripping angle brackets safely.
 */
export function formatTreasuryAction(action: string | null | undefined): string {
  if (!action || typeof action !== 'string' || !action.trim()) {
    return 'Hold / Maintain Current Allocations';
  }
  return action.trim().replace(/^<Action:\s*/, '').replace(/>$/, '').trim();
}


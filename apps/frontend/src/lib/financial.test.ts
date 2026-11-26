import { describe, it, expect } from 'vitest';
import {
  calculatePortfolioDebt,
  calculateBalanceSheet,
  getAssetExplicitValue,
  calculateCapitalAllocation
} from './financial';

describe('Financial Engine - Balance Sheet Debt Accuracy (ISS-03)', () => {
  it('deterministic test case: aggregates assets 100, 200, 300 and liabilities 50, 75', () => {
    const treasuries = [
      { id: 't-1', name: 'Treasury 1', total_capital: 100 },
      { id: 't-2', name: 'Treasury 2', total_capital: 200 },
      { id: 't-3', name: 'Treasury 3', total_capital: 300 }
    ];

    const liabilitiesMap = {
      't-1': [{ principal: 50 }],
      't-2': [{ principal: 75 }],
      't-3': [] // Zero liabilities
    };

    const { summary, portfolios } = calculateBalanceSheet(treasuries, liabilitiesMap);

    expect(summary.totalAssets).toBe(600);
    expect(summary.totalLiabilities).toBe(125);
    expect(summary.netSurplus).toBe(475);
    expect(summary.coverageRatio).toBeCloseTo(600 / 125, 4);
    expect(summary.portfolioCount).toBe(3);
    expect(summary.totalLiabilitiesCount).toBe(2);

    // Treasury-specific isolation
    expect(portfolios[0].totalAssets).toBe(100);
    expect(portfolios[0].totalLiabilities).toBe(50);
    expect(portfolios[0].netSurplus).toBe(50);

    expect(portfolios[1].totalAssets).toBe(200);
    expect(portfolios[1].totalLiabilities).toBe(75);
    expect(portfolios[1].netSurplus).toBe(125);

    expect(portfolios[2].totalAssets).toBe(300);
    expect(portfolios[2].totalLiabilities).toBe(0);
    expect(portfolios[2].netSurplus).toBe(300);
  });

  it('calculates portfolio debt with a single liability', () => {
    const liabilities = [{ principal: 15_000_000 }];
    expect(calculatePortfolioDebt(liabilities)).toBe(15_000_000);
  });

  it('calculates portfolio debt with multiple liabilities of varying amounts', () => {
    const liabilities = [
      { principal: 10_000_000 },
      { principal: 2_500_000 },
      { principal: 750_000 }
    ];
    expect(calculatePortfolioDebt(liabilities)).toBe(13_250_000);
  });

  it('handles empty liability lists producing zero', () => {
    expect(calculatePortfolioDebt([])).toBe(0);
    expect(calculatePortfolioDebt(null)).toBe(0);
    expect(calculatePortfolioDebt(undefined)).toBe(0);
  });

  it('prevents deleted liabilities from being counted (only active liabilities in list are counted)', () => {
    // When a liability is deleted, it is removed from the active list
    const activeLiabilities = [{ principal: 10_000_000 }];
    expect(calculatePortfolioDebt(activeLiabilities)).toBe(10_000_000);
  });

  it('handles missing, null, NaN, or negative principal amounts safely', () => {
    const malformed = [
      { principal: null },
      { principal: undefined },
      { principal: 'not-a-number' },
      { principal: -500_000 }, // Negative principal ignored
      { principal: 5_000_000 },
      { principal: '2500000' } // String coerced
    ];
    expect(calculatePortfolioDebt(malformed as any)).toBe(7_500_000);
  });

  it('calculates net surplus correctly when liabilities exceed assets (insolvent / deficit)', () => {
    const treasuries = [{ id: 't-distressed', name: 'Distressed Corp', total_capital: 100 }];
    const liabilitiesMap = { 't-distressed': [{ principal: 150 }] };

    const { summary, portfolios } = calculateBalanceSheet(treasuries, liabilitiesMap);
    expect(summary.totalAssets).toBe(100);
    expect(summary.totalLiabilities).toBe(150);
    expect(summary.netSurplus).toBe(-50);
    expect(portfolios[0].netSurplus).toBe(-50);
  });
});

describe('Financial Engine - Capital Allocation Weights (ISS-04)', () => {
  it('deterministic test case: three assets with values 100, 200, 300 produce exact value-weighted distribution summing to 100%', () => {
    const assets = [
      { id: 'a1', name: 'Asset 1', value: 100 },
      { id: 'a2', name: 'Asset 2', value: 200 },
      { id: 'a3', name: 'Asset 3', value: 300 }
    ];
    const capital = 600_000_000;

    const result = calculateCapitalAllocation(assets, capital);

    expect(result.mode).toBe('value-weighted');
    expect(result.label).toBe('Value-Weighted Distribution');
    expect(result.weights).toHaveLength(3);

    // Weights: 100/600 (16.67%), 200/600 (33.33%), 300/600 (50.00%)
    expect(result.weights[0]).toBeCloseTo(100 / 600, 6);
    expect(result.weights[1]).toBeCloseTo(200 / 600, 6);
    expect(result.weights[2]).toBeCloseTo(300 / 600, 6);

    // Sum of weights must equal exactly 1.0 (100%)
    expect(result.totalWeight).toBeCloseTo(1.0, 6);

    // Allocated capital amounts sum to total capital
    expect(result.allocatedAmounts[0]).toBeCloseTo(100_000_000, 2);
    expect(result.allocatedAmounts[1]).toBeCloseTo(200_000_000, 2);
    expect(result.allocatedAmounts[2]).toBeCloseTo(300_000_000, 2);
    expect(result.totalAllocated).toBeCloseTo(capital, 2);
  });

  it('extracts explicit value from various candidate fields and metadata', () => {
    expect(getAssetExplicitValue({ value: 500 })).toBe(500);
    expect(getAssetExplicitValue({ amount: 1000 })).toBe(1000);
    expect(getAssetExplicitValue({ principal: 750 })).toBe(750);
    expect(getAssetExplicitValue({ metadata: { value: 250 } })).toBe(250);
    expect(getAssetExplicitValue({ metadata: { holding_value: 350 } })).toBe(350);
    expect(getAssetExplicitValue({ value: -100 })).toBe(0);
    expect(getAssetExplicitValue({ value: 'invalid' as any })).toBe(0);
    expect(getAssetExplicitValue({} as any)).toBe(0);
  });

  it('handles empty assets array safely returning mode "empty"', () => {
    const result = calculateCapitalAllocation([], 100_000_000);
    expect(result.mode).toBe('empty');
    expect(result.weights).toEqual([]);
    expect(result.allocatedAmounts).toEqual([]);
    expect(result.totalWeight).toBe(0);
    expect(result.totalAllocated).toBe(0);
    expect(result.label).toBe('No Assets Configured');
  });

  it('handles zero total asset value safely falling back to equal-weight baseline (1/N)', () => {
    // 4 standard QUANTA assets without explicit value holdings
    const assets = [
      { id: 'gov_bond', name: 'Government Bonds' },
      { id: 'corp_bond', name: 'Corporate Bonds' },
      { id: 'equity', name: 'Equity Index' },
      { id: 'cash', name: 'Cash Equivalents' }
    ];
    const capital = 100_000_000;

    const result = calculateCapitalAllocation(assets, capital);

    expect(result.mode).toBe('equal-weight');
    expect(result.label).toBe('Equal-Weight Baseline (1/N)');
    expect(result.weights).toHaveLength(4);
    expect(result.weights[0]).toBe(0.25);
    expect(result.weights[1]).toBe(0.25);
    expect(result.weights[2]).toBe(0.25);
    expect(result.weights[3]).toBe(0.25);
    expect(result.totalWeight).toBe(1.0);
    expect(result.totalAllocated).toBe(100_000_000);
    expect(result.allocatedAmounts.every((a) => a === 25_000_000)).toBe(true);
  });

  it('handles single asset correctly assigning 100% weight', () => {
    const assets = [{ id: 'solo_asset', name: 'Solo Asset' }];
    const capital = 50_000_000;

    const result = calculateCapitalAllocation(assets, capital);
    expect(result.weights).toEqual([1.0]);
    expect(result.totalWeight).toBe(1.0);
    expect(result.allocatedAmounts).toEqual([50_000_000]);
  });

  it('handles negative, NaN, and invalid asset values appropriately', () => {
    const assets = [
      { id: 'a1', value: 100 },
      { id: 'a2', value: -50 }, // Negative value sanitized to 0
      { id: 'a3', value: 'invalid' as any }, // Invalid value sanitized to 0
      { id: 'a4', value: 300 }
    ];
    const capital = 400_000;

    const result = calculateCapitalAllocation(assets, capital);

    // Total positive value = 100 + 300 = 400
    expect(result.mode).toBe('value-weighted');
    expect(result.weights[0]).toBe(100 / 400); // 25%
    expect(result.weights[1]).toBe(0);
    expect(result.weights[2]).toBe(0);
    expect(result.weights[3]).toBe(300 / 400); // 75%
    expect(result.totalWeight).toBe(1.0);
    expect(result.allocatedAmounts[0]).toBe(100_000);
    expect(result.allocatedAmounts[3]).toBe(300_000);
  });

  it('handles zero or negative capital safely', () => {
    const assets = [{ id: 'a1', value: 100 }, { id: 'a2', value: 200 }];
    const resultZero = calculateCapitalAllocation(assets, 0);
    expect(resultZero.weights).toHaveLength(2);
    expect(resultZero.totalWeight).toBe(1.0);
    expect(resultZero.totalAllocated).toBe(0);

    const resultNeg = calculateCapitalAllocation(assets, -500);
    expect(resultNeg.totalAllocated).toBe(0);
  });
});

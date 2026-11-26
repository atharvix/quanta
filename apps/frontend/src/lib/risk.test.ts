import { describe, it, expect } from 'vitest';
import {
  parseAllocations,
  extractAllocations,
  calculateLiquidityFloor,
  formatBayesianEvidence,
  getRiskTierClass,
  formatTreasuryAction
} from './risk';

describe('QUANTA F5 Frontend Risk Logic', () => {
  // =========================================================================
  // Section A: Allocation Extraction
  // =========================================================================
  describe('A. Allocation Extraction', () => {
    const catalog = [
      { id: 'gov_bond', name: 'Government Bonds' },
      { id: 'bluechip_equity', name: 'Bluechip Equity' },
      { id: 'corp_paper', name: 'Commercial Paper' }
    ];

    it('extracts allocations from a valid JSON string', () => {
      const raw = JSON.stringify({ gov_bond: 50.0, bluechip_equity: 30.0 });
      const result = extractAllocations(raw, catalog);

      expect(result).toEqual({
        gov_bond: 50.0,
        bluechip_equity: 30.0
      });
    });

    it('extracts allocations from an already parsed allocation object', () => {
      const raw = { gov_bond: 60.0, corp_paper: 25.0 };
      const result = extractAllocations(raw, catalog);

      expect(result).toEqual({
        gov_bond: 60.0,
        corp_paper: 25.0
      });
    });

    it('returns empty dictionary for empty allocation object or string', () => {
      expect(extractAllocations({}, catalog)).toEqual({});
      expect(extractAllocations('{}', catalog)).toEqual({});
      expect(extractAllocations([], catalog)).toEqual({});
    });

    it('safely handles missing, null, or undefined allocation values', () => {
      expect(extractAllocations(null, catalog)).toEqual({});
      expect(extractAllocations(undefined, catalog)).toEqual({});
      expect(extractAllocations('', catalog)).toEqual({});
    });

    it('safely handles malformed and invalid JSON without crashing', () => {
      const invalidJson = '{ malformed: json, missing_quotes: 123 ';
      expect(extractAllocations(invalidJson, catalog)).toEqual({});
    });

    it('filters out asset IDs that were removed from the active investment catalog', () => {
      const raw = {
        gov_bond: 40.0,
        removed_foreign_asset: 20.0,
        deleted_token: 10.0
      };
      const result = extractAllocations(raw, catalog);

      expect(result).toEqual({
        gov_bond: 40.0
      });
      expect(result).not.toHaveProperty('removed_foreign_asset');
      expect(result).not.toHaveProperty('deleted_token');
    });

    it('excludes synthetic reserved artifact keys such as "raw" and "value"', () => {
      const malformedArray = ['corrupt_string_item', 12345];
      const result = extractAllocations(malformedArray, catalog);

      expect(result).not.toHaveProperty('raw');
      expect(result).not.toHaveProperty('value');
      expect(result).not.toHaveProperty('Item 1');
    });

    it('extracts allocations from historical saved strategy log objects', () => {
      const savedStrategy = {
        id: 'strat_log_001',
        algorithm_used: 'astar',
        allocations_json: { gov_bond: 70.0, corp_paper: 15.0 }
      };

      const result = extractAllocations(savedStrategy.allocations_json, catalog);
      expect(result).toEqual({
        gov_bond: 70.0,
        corp_paper: 15.0
      });
    });

    it('extracts allocations from active strategy orchestration responses (array of objects representation)', () => {
      const rawArray = [
        { asset: 'gov_bond', amount: 55.0 },
        { asset: 'bluechip_equity', amount: 25.0 }
      ];

      const result = extractAllocations(rawArray, catalog);
      expect(result).toEqual({
        gov_bond: 55.0,
        bluechip_equity: 25.0
      });
    });

    it('parses allocations correctly via parseAllocations for display helpers', () => {
      const parsed = parseAllocations({ gov_bond: 40.0 });
      expect(parsed).toHaveLength(1);
      expect(parsed[0].asset).toBe('gov_bond');
      expect(parsed[0].amount).toBe(40.0);
      expect(parsed[0].formattedAmount).toBeDefined();
    });
  });

  // =========================================================================
  // Section B: Liquidity Floor
  // =========================================================================
  describe('B. Liquidity Floor', () => {
    it('preserves an explicit zero string as 0 without defaulting to fallback', () => {
      const floor = calculateLiquidityFloor('0', 0.20);
      expect(floor).toBe(0);
    });

    it('preserves an explicit zero number as 0', () => {
      const floor = calculateLiquidityFloor(0, 0.25);
      expect(floor).toBe(0);
    });

    it('preserves an explicit decimal zero "0.0" as 0', () => {
      const floor = calculateLiquidityFloor('0.0', 0.20);
      expect(floor).toBe(0);
    });

    it('preserves positive configured threshold unchanged', () => {
      expect(calculateLiquidityFloor('0.35', 0.20)).toBe(0.35);
      expect(calculateLiquidityFloor(0.15, 0.20)).toBe(0.15);
      expect(calculateLiquidityFloor('0.05', 0.20)).toBe(0.05);
    });

    it('uses treasury default when input is missing or empty string', () => {
      expect(calculateLiquidityFloor('', 0.18)).toBe(0.18);
      expect(calculateLiquidityFloor(null, 0.25)).toBe(0.25);
      expect(calculateLiquidityFloor(undefined, 0.30)).toBe(0.30);
    });

    it('uses established fallback 0.20 when both input and treasury default are missing', () => {
      expect(calculateLiquidityFloor(null, null)).toBe(0.20);
      expect(calculateLiquidityFloor(undefined, undefined)).toBe(0.20);
      expect(calculateLiquidityFloor('', null)).toBe(0.20);
    });

    it('uses treasury default or fallback when input is invalid non-numeric string', () => {
      expect(calculateLiquidityFloor('invalid_input', 0.25)).toBe(0.25);
      expect(calculateLiquidityFloor('NaN', null)).toBe(0.20);
    });

    it('confirms absence of accidental || falsy fallback bug on zero', () => {
      // Common JavaScript bug: parseFloat("0") || 0.20 results in 0.20.
      // calculateLiquidityFloor must strictly return 0.
      const flawedImplementation = parseFloat('0') || 0.20;
      expect(flawedImplementation).toBe(0.20); // illustrates the bug

      const correctResult = calculateLiquidityFloor('0', 0.20);
      expect(correctResult).toBe(0); // proves bug is avoided
    });
  });

  // =========================================================================
  // Section C: Bayesian Evidence
  // =========================================================================
  describe('C. Bayesian Evidence', () => {
    it('returns undefined when all signals are unobserved so property is omitted in payload', () => {
      const evidence = formatBayesianEvidence('unobserved', 'unobserved', 'unobserved');
      expect(evidence).toBeUndefined();

      const payload = {
        assets: [],
        evidence
      };
      const serialized = JSON.stringify(payload);
      expect(serialized).not.toContain('"evidence"');
    });

    it('correctly sets all signals to true when observed active', () => {
      const evidence = formatBayesianEvidence('true', 'true', 'true');
      expect(evidence).toEqual({
        inflation_high: true,
        rate_hike: true,
        recession_signal: true
      });
    });

    it('correctly sets all signals to false when observed inactive', () => {
      const evidence = formatBayesianEvidence('false', 'false', 'false');
      expect(evidence).toEqual({
        inflation_high: false,
        rate_hike: false,
        recession_signal: false
      });
    });

    it('correctly handles mixed combinations of true, false, and unobserved', () => {
      const evidence = formatBayesianEvidence('true', 'unobserved', 'false');
      expect(evidence).toBeDefined();
      expect(evidence?.inflation_high).toBe(true);
      expect(evidence?.recession_signal).toBe(false);
      expect(evidence).not.toHaveProperty('rate_hike');
    });

    it('omits unobserved properties from serialized JSON payload', () => {
      const evidence = formatBayesianEvidence('unobserved', 'true', 'unobserved');
      expect(evidence).toBeDefined();
      const serialized = JSON.stringify(evidence);

      expect(serialized).toContain('"rate_hike":true');
      expect(serialized).not.toContain('inflation_high');
      expect(serialized).not.toContain('recession_signal');
    });
  });

  // =========================================================================
  // Section D: Risk Tier and Action Formatting
  // =========================================================================
  describe('D. Risk Tier and Action Formatting', () => {
    describe('Risk Tier CSS Class Mapping', () => {
      it('maps low and conservative tiers to badge-tier-low', () => {
        expect(getRiskTierClass('Low')).toBe('badge-tier-low');
        expect(getRiskTierClass('low')).toBe('badge-tier-low');
        expect(getRiskTierClass('Ultra-Conservative')).toBe('badge-tier-low');
        expect(getRiskTierClass('Conservative')).toBe('badge-tier-low');
      });

      it('maps moderate tiers to badge-tier-moderate', () => {
        expect(getRiskTierClass('Moderate')).toBe('badge-tier-moderate');
        expect(getRiskTierClass('moderate')).toBe('badge-tier-moderate');
        expect(getRiskTierClass('MODERATE RISK')).toBe('badge-tier-moderate');
      });

      it('maps high, critical, and severe tiers to badge-tier-high', () => {
        expect(getRiskTierClass('High')).toBe('badge-tier-high');
        expect(getRiskTierClass('Critical')).toBe('badge-tier-high');
        expect(getRiskTierClass('Severe')).toBe('badge-tier-high');
        expect(getRiskTierClass('Aggressive')).toBe('badge-tier-high');
      });

      it('safely handles unknown, empty, or null risk tiers without crashing', () => {
        expect(getRiskTierClass('')).toBe('badge-tier-moderate');
        expect(getRiskTierClass(null)).toBe('badge-tier-moderate');
        expect(getRiskTierClass(undefined)).toBe('badge-tier-moderate');
        expect(getRiskTierClass('CustomUnknownTier')).toBe('badge-tier-moderate');
      });
    });

    describe('Defensive Action String Formatting', () => {
      it('formats Python Action representation strings by stripping angle brackets and Action prefix', () => {
        const raw = '<Action: INVEST ₹5.00 -> bluechip_equity>';
        expect(formatTreasuryAction(raw)).toBe('INVEST ₹5.00 -> bluechip_equity');
      });

      it('formats simple Action tags without parameters', () => {
        const raw = '<Action: HOLD_LIQUID_RESERVES>';
        expect(formatTreasuryAction(raw)).toBe('HOLD_LIQUID_RESERVES');
      });

      it('returns plain strings unchanged if no Action angle bracket prefix exists', () => {
        const plain = 'Rebalance portfolio towards short-term debt';
        expect(formatTreasuryAction(plain)).toBe('Rebalance portfolio towards short-term debt');
      });

      it('handles empty, null, and undefined strings safely with default guidance', () => {
        expect(formatTreasuryAction(null)).toBe('Hold / Maintain Current Allocations');
        expect(formatTreasuryAction(undefined)).toBe('Hold / Maintain Current Allocations');
        expect(formatTreasuryAction('')).toBe('Hold / Maintain Current Allocations');
        expect(formatTreasuryAction('   ')).toBe('Hold / Maintain Current Allocations');
      });
    });
  });
});


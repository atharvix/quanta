import { describe, it, expect } from 'vitest';
import {
  escapeCSVCell,
  generateBalanceSheetCSV,
  generateAuditTrailCSV,
  generateJSONReport
} from './reports';
import type { Treasury, AuditLogRecord } from './types';

describe('Reports Generation & RFC 4180 CSV Escaping', () => {
  it('escapes null and undefined values as empty string', () => {
    expect(escapeCSVCell(null)).toBe('');
    expect(escapeCSVCell(undefined)).toBe('');
  });

  it('preserves clean numeric and alphanumeric strings unquoted', () => {
    expect(escapeCSVCell('Apex Capital')).toBe('Apex Capital');
    expect(escapeCSVCell(15000000)).toBe('15000000');
    expect(escapeCSVCell('USD')).toBe('USD');
  });

  it('escapes cells containing commas with double quotes', () => {
    expect(escapeCSVCell('New York, NY')).toBe('"New York, NY"');
    expect(escapeCSVCell('1,000,000.00')).toBe('"1,000,000.00"');
  });

  it('escapes cells containing double quotes by doubling them and wrapping', () => {
    expect(escapeCSVCell('Senior "Revolving" Facility')).toBe('"Senior ""Revolving"" Facility"');
    expect(escapeCSVCell('"Quoted"')).toBe('"""Quoted"""');
  });

  it('escapes cells containing newlines and carriage returns', () => {
    expect(escapeCSVCell("Line 1\nLine 2")).toBe('"Line 1\nLine 2"');
    expect(escapeCSVCell("Line 1\r\nLine 2")).toBe('"Line 1\r\nLine 2"');
  });

  it('serializes structured objects and maps safely to escaped JSON strings', () => {
    const meta = { risk_tier: 'Low', score: 0.15 };
    const escaped = escapeCSVCell(meta);
    expect(escaped).toContain('""risk_tier"":""Low""');
    expect(escaped.startsWith('"')).toBe(true);
    expect(escaped.endsWith('"')).toBe(true);
  });

  it('generates a complete balance sheet CSV with correct headers and CRLF line breaks', () => {
    const treasuries: Treasury[] = [
      {
        id: '41002f69-419b-43f1-a82e-c730db41eb81',
        name: 'Apex Institutional Capital, LLC',
        base_currency: 'USD',
        total_capital: 150000000,
        risk_tolerance: 0.65,
        min_liquidity_ratio: 0.25,
        created_at: '2026-11-20T00:00:00Z',
        updated_at: '2026-11-20T00:00:00Z'
      },
      {
        id: '52002f69-419b-43f1-a82e-c730db41eb82',
        name: 'Global "Yield" Portfolio',
        base_currency: 'EUR',
        total_capital: 85000000,
        risk_tolerance: 0.45,
        min_liquidity_ratio: 0.20,
        created_at: '2026-11-21T00:00:00Z',
        updated_at: '2026-11-21T00:00:00Z'
      }
    ];

    const csv = generateBalanceSheetCSV(treasuries);
    const lines = csv.split('\r\n');
    expect(lines.length).toBe(3);
    expect(lines[0]).toBe('Portfolio ID,Portfolio Name,Base Currency,Total Capital,Risk Tolerance,Min Liquidity Ratio,Created At,Updated At');
    expect(lines[1]).toContain('"Apex Institutional Capital, LLC"');
    expect(lines[2]).toContain('"Global ""Yield"" Portfolio"');
  });

  it('generates an audit trail CSV handling empty and populated metadata', () => {
    const logs: AuditLogRecord[] = [
      {
        id: 'log-1',
        action: 'auth.login',
        actor: 'admin@quanta.local',
        entity_type: 'user',
        entity_id: 'user-1',
        created_at: '2026-11-26T12:00:00Z',
        metadata: { ip: '127.0.0.1' }
      },
      {
        id: 'log-2',
        action: 'treasury.created',
        actor: 'admin@quanta.local',
        entity_type: 'treasury',
        entity_id: 'treasury-1',
        created_at: '2026-11-26T12:05:00Z',
        metadata: null
      }
    ];

    const csv = generateAuditTrailCSV(logs);
    const lines = csv.split('\r\n');
    expect(lines.length).toBe(3);
    expect(lines[0]).toBe('Log ID,Timestamp,Action,Entity Type,Entity ID,Actor,Metadata');
    expect(lines[1]).toContain('auth.login');
    expect(lines[2]).toContain('treasury.created');
  });

  it('generates valid JSON report matching source data', () => {
    const data = [{ id: 1, name: 'Test' }];
    const json = generateJSONReport(data);
    expect(JSON.parse(json)).toEqual(data);
    expect(json).toContain('"id": 1');
  });

  it('returns empty string for empty input datasets', () => {
    expect(generateBalanceSheetCSV([])).toBe('');
    expect(generateAuditTrailCSV([])).toBe('');
    expect(generateJSONReport([])).toBe('[]');
  });
});

import type { Treasury, AuditLogRecord } from './types';

/**
 * Escapes a cell value strictly according to RFC 4180 rules:
 * - Null or undefined values produce an empty string.
 * - Objects and arrays are serialized to JSON.
 * - If string contains comma, quote, carriage return, or newline, wraps in double quotes and escapes quotes as "".
 */
export function escapeCSVCell(val: unknown): string {
  if (val === null || val === undefined) {
    return '';
  }

  let str: string;
  if (typeof val === 'object') {
    try {
      str = JSON.stringify(val);
    } catch {
      str = String(val);
    }
  } else {
    str = String(val);
  }

  if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  return str;
}

/**
 * Generates an RFC 4180 compliant CSV string from an array of Treasury records.
 * Encodes all available records without truncation.
 */
export function generateBalanceSheetCSV(treasuries: Treasury[]): string {
  if (!treasuries || treasuries.length === 0) {
    return '';
  }

  const headers = [
    'Portfolio ID',
    'Portfolio Name',
    'Base Currency',
    'Total Capital',
    'Risk Tolerance',
    'Min Liquidity Ratio',
    'Created At',
    'Updated At'
  ];

  const rows = treasuries.map((t) => [
    escapeCSVCell(t.id),
    escapeCSVCell(t.name),
    escapeCSVCell(t.base_currency || 'USD'),
    escapeCSVCell(t.total_capital),
    escapeCSVCell(t.risk_tolerance),
    escapeCSVCell(t.min_liquidity_ratio),
    escapeCSVCell(t.created_at),
    escapeCSVCell(t.updated_at)
  ]);

  return [
    headers.join(','),
    ...rows.map((row) => row.join(','))
  ].join('\r\n');
}

/**
 * Generates an RFC 4180 compliant CSV string from an array of Audit Log records.
 * Encodes all available records including structured JSON metadata without truncation.
 */
export function generateAuditTrailCSV(logs: AuditLogRecord[]): string {
  if (!logs || logs.length === 0) {
    return '';
  }

  const headers = [
    'Log ID',
    'Timestamp',
    'Action',
    'Entity Type',
    'Entity ID',
    'Actor',
    'Metadata'
  ];

  const rows = logs.map((log) => [
    escapeCSVCell(log.id),
    escapeCSVCell(log.created_at),
    escapeCSVCell(log.action),
    escapeCSVCell(log.entity_type),
    escapeCSVCell(log.entity_id || ''),
    escapeCSVCell(log.actor),
    escapeCSVCell(log.metadata || {})
  ]);

  return [
    headers.join(','),
    ...rows.map((row) => row.join(','))
  ].join('\r\n');
}

/**
 * Generates formatted, indented JSON for enterprise reporting.
 */
export function generateJSONReport(data: unknown): string {
  if (data === null || data === undefined) {
    return '[]';
  }
  return JSON.stringify(data, null, 2);
}

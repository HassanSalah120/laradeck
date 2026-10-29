export function formatRecordValue(value: unknown): string {
  if (value === null || value === undefined) return '';
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

// Only changed fields cross the write boundary. Keep PHP/Eloquent value types.
export function parseRecordChanges(
  original: Record<string, unknown>,
  draft: Record<string, unknown>,
  casts: Record<string, string>,
  primaryKey = 'id',
): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [field, raw] of Object.entries(draft)) {
    if ([primaryKey, 'created_at', 'updated_at', 'deleted_at', 'creation_date', 'modified_date'].includes(field)) continue;
    const previous = original[field];
    if (raw === previous) continue;
    if (raw !== null && raw === formatRecordValue(previous) && previous !== null) continue;
    if (raw === null) { result[field] = null; continue; }
    const text = String(raw);
    const cast = (casts[field] || '').toLowerCase().split(':');
    const type = cast[0] === 'encrypted' ? cast[1] : cast[0];
    const boolean = ['bool', 'boolean'].includes(type) || typeof previous === 'boolean';
    const json = ['array', 'json', 'object', 'collection'].includes(type) || (previous !== null && typeof previous === 'object');
    const numeric = ['int', 'integer', 'float', 'double', 'real'].includes(type) || typeof previous === 'number';
    if (boolean) {
      if (text === 'true' || text === '1') result[field] = true;
      else if (text === 'false' || text === '0') result[field] = false;
      else throw new Error(`${field}: enter true or false, or select NULL`);
    } else if (json) {
      let parsed: unknown;
      try { parsed = JSON.parse(text); } catch { throw new Error(`${field}: enter valid JSON`); }
      if (parsed !== null && typeof parsed !== 'object') throw new Error(`${field}: expected a JSON object or array`);
      result[field] = parsed;
    } else if (numeric) {
      const number = Number(text);
      if (!text.trim() || !Number.isFinite(number)) throw new Error(`${field}: enter a valid number or select NULL`);
      if ((['int', 'integer'].includes(type) || Number.isInteger(number)) && !Number.isSafeInteger(number)) {
        throw new Error(`${field}: enter a safe integer`);
      }
      result[field] = number;
    } else {
      result[field] = text;
    }
  }
  return result;
}

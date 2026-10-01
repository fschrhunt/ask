/*
 * The JSON Schema check ask applies to --schema answers, whatever the agent. It covers the
 * keywords answers rely on: type, enum, const, properties, required, additionalProperties false and
 * items. Other keywords are not checked.
 */

/* Deep equality for JSON values; object key order does not matter. */
function same(a, b) {
  if (a === b) return true;
  if (typeof a !== 'object' || typeof b !== 'object' || a === null || b === null || Array.isArray(a) !== Array.isArray(b)) return false;
  const keys = Object.keys(a);
  return keys.length === Object.keys(b).length && keys.every((key) => Object.hasOwn(b, key) && same(a[key], b[key]));
}

/* Returns the first mismatch as "path: problem", or '' when the value matches. */
export function schemaMismatch(value, schema, path = '$') {
  if (schema === false) return `${path}: no value is allowed here`;
  if (!schema || typeof schema !== 'object') return '';
  const kind = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value;
  const fits = (type) => type === kind || (type === 'integer' && Number.isInteger(value));
  const types = [schema.type].flat().filter(Boolean);
  if (types.length && !types.some(fits)) return `${path}: expected ${types.join(' or ')}, got ${kind}`;
  if (schema.enum && !schema.enum.some((option) => same(option, value))) return `${path}: not one of ${JSON.stringify(schema.enum)}`;
  if ('const' in schema && !same(schema.const, value)) return `${path}: must be ${JSON.stringify(schema.const)}`;
  if (kind === 'object') {
    for (const key of schema.required || []) if (!Object.hasOwn(value, key)) return `${path}: missing "${key}"`;
    for (const [key, item] of Object.entries(value)) {
      if (schema.properties && Object.hasOwn(schema.properties, key)) {
        const problem = schemaMismatch(item, schema.properties[key], `${path}.${key}`);
        if (problem) return problem;
      } else if (schema.additionalProperties === false) return `${path}: unexpected "${key}"`;
    }
  }
  if (kind === 'array' && schema.items)
    for (const [index, item] of value.entries()) {
      const problem = schemaMismatch(item, schema.items, `${path}[${index}]`);
      if (problem) return problem;
    }
  return '';
}

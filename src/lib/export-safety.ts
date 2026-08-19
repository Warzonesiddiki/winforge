/** Escape untrusted text before interpolating it into an HTML document. */
export function escapeHtml(value: unknown): string {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

/**
 * Encode a CSV cell and neutralize spreadsheet formulas.
 *
 * History exports are commonly opened in Excel or LibreOffice. Those programs
 * may execute cells beginning with =, +, -, or @ as formulas, so prefix such
 * values with an apostrophe before applying RFC 4180 quoting.
 */
export function csvEscape(value: unknown): string {
  let text = String(value);
  if (/^[\t\r ]*[=+\-@]/.test(text)) {
    text = `'${text}`;
  }
  if (/[",\r\n]/.test(text)) {
    return `"${text.replaceAll('"', '""')}"`;
  }
  return text;
}

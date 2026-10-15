/**
 * Knowledge Engine & Metric Detection
 *
 * Detects and replaces unverified metric placeholders ([METRIC: ...])
 * in synthesized resume drafts.
 */

export function detectMetrics(markdown: string): string[] {
    if (!markdown) return [];
    const matches = markdown.match(/\[METRIC:[^\]]+\]/g);
    if (!matches) return [];
    return Array.from(new Set(matches));
}

export function replaceMetric(markdown: string, token: string, value: string): string {
    if (!markdown || !token) return markdown;
    return markdown.split(token).join(value);
}

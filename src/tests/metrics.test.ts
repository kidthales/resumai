import { describe, expect, it } from 'vitest';
import { detectMetrics, replaceMetric } from '../lib/metrics';

describe('Suite 5: Metric Placeholder Detection & Replacement', () => {
    it('TC-5.1: detects unique [METRIC: ...] placeholders in markdown text', () => {
        const text = `
- Reduced latency by [METRIC: e.g. 45% reduction in latency/faults] across all nodes.
- Saved [METRIC: e.g. $50k monthly savings] in cloud spend.
- Repeated token [METRIC: e.g. 45% reduction in latency/faults] should be deduplicated.
`;
        const metrics = detectMetrics(text);
        expect(metrics).toHaveLength(2);
        expect(metrics).toEqual([
            '[METRIC: e.g. 45% reduction in latency/faults]',
            '[METRIC: e.g. $50k monthly savings]'
        ]);
    });

    it('TC-5.1: returns empty array when no metric tokens exist', () => {
        const text = '- Standard accomplishment without any placeholder tokens.';
        expect(detectMetrics(text)).toEqual([]);
        expect(detectMetrics('')).toEqual([]);
    });

    it('TC-5.2: replaces target metric token across entire markdown text', () => {
        const draft = 'Achieved [METRIC: % latency drop] and verified [METRIC: % latency drop] in staging.';
        const updated = replaceMetric(draft, '[METRIC: % latency drop]', '38% drop in P99 latency');
        expect(updated).toBe('Achieved 38% drop in P99 latency and verified 38% drop in P99 latency in staging.');
        expect(detectMetrics(updated)).toEqual([]);
    });

    it('TC-5.2: gracefully returns original string if token is missing or empty', () => {
        const draft = 'Original text';
        expect(replaceMetric(draft, '', 'value')).toBe('Original text');
        expect(replaceMetric('', '[METRIC: x]', 'value')).toBe('');
    });
});

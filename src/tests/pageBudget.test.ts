import { describe, expect, it } from 'vitest';
import { calculatePageBudget, LETTER_PAGE_PX } from '../lib/pageBudget';

describe('Suite 6: Page-Budget Engine (Letter 1056px at 96 CSS DPI)', () => {
    it('constant matches 11 inches at 96 CSS DPI (1056px)', () => {
        expect(LETTER_PAGE_PX).toBe(1056);
    });

    it('TC-6.2: calculates optimal budget for sub-1-page content (~0.95 pages)', () => {
        const result = calculatePageBudget(1003); // ~0.95 pages
        expect(result.pages).toBeCloseTo(0.95, 2);
        expect(result.pagesFormatted).toBe('0.95');
        expect(result.isSpillover).toBe(false);
        expect(result.label).toBe('0.95 / 2.00 pages (Optimal)');
    });

    it('TC-6.2: flags overflow warning for awkward trailing spillover (~1.12 pages)', () => {
        const result = calculatePageBudget(1183); // ~1.12 pages
        expect(result.pages).toBeCloseTo(1.12, 2);
        expect(result.pagesFormatted).toBe('1.12');
        expect(result.isSpillover).toBe(true);
        expect(result.label).toBe('1.12 pages (Overflow Warning!)');
    });

    it('TC-6.2: calculates optimal budget for clean 2-page resumes (~1.90 pages)', () => {
        const result = calculatePageBudget(2006); // ~1.90 pages
        expect(result.pages).toBeCloseTo(1.9, 2);
        expect(result.pagesFormatted).toBe('1.90');
        expect(result.isSpillover).toBe(false);
        expect(result.label).toBe('1.90 / 2.00 pages (Optimal)');
    });

    it('TC-6.2: flags overflow warning when exceeding 2 pages (~2.08 pages)', () => {
        const result = calculatePageBudget(2196); // ~2.08 pages
        expect(result.pages).toBeCloseTo(2.08, 2);
        expect(result.pagesFormatted).toBe('2.08');
        expect(result.isSpillover).toBe(true);
        expect(result.label).toBe('2.08 pages (Overflow Warning!)');
    });

    it('clamps minimum page budget to 0.10 for empty content', () => {
        const result = calculatePageBudget(0);
        expect(result.pages).toBe(0.1);
        expect(result.pagesFormatted).toBe('0.10');
        expect(result.isSpillover).toBe(false);
    });
});

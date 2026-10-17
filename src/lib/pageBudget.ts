/**
 * Page-Budget Engine
 *
 * Computes Letter (8.5in x 11in at 96 CSS DPI = 1056px height) page
 * budget occupancy, line overflow/underflow deltas, and flags trailing page overflow warnings.
 */

export const LETTER_PAGE_PX = 1056;
export const DEFAULT_LINE_HEIGHT_PX = 18;

export interface PageBudgetResult {
    pages: number;
    pagesFormatted: string;
    targetPages: 1 | 2;
    excessPx: number;
    deltaLines: number;
    isSpillover: boolean;
    needsRefinement: boolean;
    label: string;
}

export function calculatePageBudget(
    totalHeightPx: number,
    targetPagesOverride?: 1 | 2,
    lineHeightPx: number = DEFAULT_LINE_HEIGHT_PX
): PageBudgetResult {
    const rawPages = Math.max(0.1, totalHeightPx / LETTER_PAGE_PX);
    const pagesFormatted = rawPages.toFixed(2);

    const targetPages: 1 | 2 = targetPagesOverride ?? (rawPages <= 1.25 ? 1 : 2);
    const targetHeightPx = targetPages * LETTER_PAGE_PX;

    const excessPx = Math.round(totalHeightPx - targetHeightPx);
    const deltaLines = Math.round(excessPx / lineHeightPx);

    const isSpillover = (rawPages > 1.02 && rawPages < 1.25) || rawPages > 2.02;
    const needsRefinement = excessPx > 15 || (targetPages === 2 && deltaLines < -15);

    let label: string;
    if (isSpillover) {
        label = `${pagesFormatted} pages (Overflow Warning!)`;
    } else {
        label = `${pagesFormatted} / 2.00 pages (Optimal)`;
    }

    return {
        pages: rawPages,
        pagesFormatted,
        targetPages,
        excessPx,
        deltaLines,
        isSpillover,
        needsRefinement,
        label
    };
}

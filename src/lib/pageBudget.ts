/**
 * Page-Budget Engine
 *
 * Computes Letter (8.5in x 11in at 96 CSS DPI = 1056px height) page
 * budget occupancy and flags trailing page overflow warnings.
 */

export const LETTER_PAGE_PX = 1056;

export interface PageBudgetResult {
    pages: number;
    pagesFormatted: string;
    isSpillover: boolean;
    label: string;
}

export function calculatePageBudget(totalHeightPx: number): PageBudgetResult {
    const pages = Math.max(0.1, totalHeightPx / LETTER_PAGE_PX);
    const pagesFormatted = pages.toFixed(2);
    const isSpillover = (pages > 1.02 && pages < 1.25) || pages > 2.02;

    const label = isSpillover
        ? `${pagesFormatted} pages (Overflow Warning!)`
        : `${pagesFormatted} / 2.00 pages (Optimal)`;

    return {
        pages,
        pagesFormatted,
        isSpillover,
        label
    };
}

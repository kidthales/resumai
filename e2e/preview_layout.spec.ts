import { expect, test } from '@playwright/test';

test.describe('Suite 6: Real Browser Layout, Typography Sliders & Page Budgeting', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await page.waitForSelector('#preview-iframe');
    });

    test('TC-6.1: renders initial sample resume inside live preview iframe', async ({ page }) => {
        const iframeElement = page.frameLocator('#preview-iframe');
        // Check that H1 header rendered from markdown
        const h1 = iframeElement.locator('h1');
        await expect(h1).toContainText('Dr. Henry "Hank" McCoy');

        // Check professional experience section rendered
        const expHeader = iframeElement.locator('h2', { hasText: 'Professional Experience' });
        await expect(expHeader).toBeVisible();
    });

    test('TC-6.2: calculates Letter page budget pill and transitions between overflow warning and optimal', async ({
        page
    }) => {
        const budgetPill = page.locator('#page-budget-pill');
        await expect(budgetPill).toBeVisible();

        // Initial sample resume at default 10pt font & 0.45in margin lands at ~1.05 pages (trailing overflow warning)
        await expect(budgetPill).toHaveClass(/budget-pill warning/);
        await expect(budgetPill).toContainText('(Overflow Warning!)');

        // Adjust font slider down to 9.25pt to fit comfortably on 1 Letter page
        const fontSlider = page.locator('#slider-font-size');
        await fontSlider.fill('9.25');
        await fontSlider.dispatchEvent('input');

        // Verify budget pill recalculates dynamically to Optimal
        await expect(budgetPill).toHaveClass(/budget-pill optimal/);
        await expect(budgetPill).toContainText('/ 2.00 pages (Optimal)');
    });

    test('TC-6.3: live typography sliders update labels and re-render preview', async ({ page }) => {
        // Adjust Font Size slider
        const fontSlider = page.locator('#slider-font-size');
        await fontSlider.fill('11');
        await fontSlider.dispatchEvent('input');

        const fontLabel = page.locator('#lbl-font-size');
        await expect(fontLabel).toHaveText('11pt');

        // Adjust Line Height slider
        const lineSlider = page.locator('#slider-line-height');
        await lineSlider.fill('1.45');
        await lineSlider.dispatchEvent('input');

        const lineLabel = page.locator('#lbl-line-height');
        await expect(lineLabel).toHaveText('1.45');

        // Adjust Margin slider
        const marginSlider = page.locator('#slider-margin');
        await marginSlider.fill('0.55');
        await marginSlider.dispatchEvent('input');

        const marginLabel = page.locator('#lbl-margin');
        await expect(marginLabel).toHaveText('0.55in');

        // Verify page budget pill re-computed
        const budgetPill = page.locator('#page-budget-pill');
        await expect(budgetPill).toBeVisible();
    });

    test('TC-2.1: settings modal opens and closes in real Chromium', async ({ page }) => {
        const settingsBtn = page.locator('#btn-open-settings');
        await settingsBtn.click();

        const modal = page.locator('#settings-modal');
        await expect(modal).toBeVisible();

        const closeBtn = page.locator('#btn-close-settings');
        await closeBtn.click();

        await expect(modal).toHaveClass(/hidden/);
    });
});

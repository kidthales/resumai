import fs from 'node:fs';
import path from 'node:path';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { mockIPC } from '@tauri-apps/api/mocks';
import { beforeEach, describe, expect, it } from 'vitest';
import App from '../App.svelte';
import { state } from '../lib/state.svelte';

describe('Suite 10: Svelte 5 (Runes) Reactive Components & @kidthales/picotol Integration', () => {
    beforeEach(() => {
        state.reset();
        mockIPC((cmd) => {
            if (cmd === 'get_vault_overview') {
                return {
                    root_path: 'C:/TestVault',
                    pii_files: [],
                    archetype_files: [],
                    style_files: ['resume.sample.css'],
                    prompt_files: [],
                    applications: []
                };
            }
            if (cmd === 'get_default_llm_config') {
                return {
                    platform: 'ollama',
                    ollama_base_url: 'http://localhost:11434',
                    ollama_model: 'qwen2.5:14b-instruct-q5_K_M',
                    gemini_api_key: '',
                    gemini_flash_model: 'gemini-3.5-flash',
                    gemini_lite_model: 'gemini-3.5-flash-lite'
                };
            }
            if (cmd === 'read_style_css') {
                return 'body { font-family: sans-serif; }';
            }
        });
    });

    it('TC-10.1: verifies @kidthales/picotol tokens and palettes are imported in main.css', () => {
        const cssPath = path.resolve(__dirname, '../styles/main.css');
        const cssContent = fs.readFileSync(cssPath, 'utf-8');
        expect(cssContent).toContain("@import '@kidthales/picotol/tokens';");
        expect(cssContent).toContain("@import '@kidthales/picotol/palettes';");
        expect(cssContent).toContain('--tol-vibrant-blue');
        expect(cssContent).toContain('--tol-bright-cyan');
        expect(cssContent).toContain('--tol-light-mint');
        expect(cssContent).toContain('--tol-hc-yellow');
        expect(cssContent).toContain('--tol-bright-red');
    });

    it('TC-10.2: resizes sidebar and preview panes via SplitPane keyboard separators and persists to localStorage', async () => {
        render(App);
        await state.init();

        const leftGutter = screen.getByLabelText('Resize Vault Sidebar');
        const rightGutter = screen.getByLabelText('Resize Paper Preview');

        expect(state.sidebarWidthPx).toBe(260);
        expect(state.previewWidthPx).toBe(520);

        await fireEvent.keyDown(leftGutter, { key: 'ArrowRight' });
        expect(state.sidebarWidthPx).toBe(276);

        await fireEvent.keyDown(rightGutter, { key: 'ArrowLeft' });
        expect(state.previewWidthPx).toBe(536);

        const saved = JSON.parse(localStorage.getItem('resumai_pane_widths') || '{}');
        expect(saved.sidebar).toBe(276);
        expect(saved.preview).toBe(536);
    });

    it('TC-10.3: synchronizes typography numeric steppers 2-way with range sliders and state', async () => {
        render(App);
        await state.init();

        const fontStepper = screen.getByLabelText('Font size numeric stepper') as HTMLInputElement;
        await fireEvent.input(fontStepper, { target: { value: '10.75' } });
        expect(state.fontSizePt).toBe(10.75);
        expect(document.getElementById('lbl-font-size')?.textContent).toBe('10.75pt');

        const marginStepper = screen.getByLabelText('Margin numeric stepper') as HTMLInputElement;
        await fireEvent.input(marginStepper, { target: { value: '0.55' } });
        expect(state.marginIn).toBe(0.55);
        expect(document.getElementById('lbl-margin')?.textContent).toBe('0.55in');
    });

    it('TC-10.4: reactively detects [METRIC: ...] placeholders and replaces inline via MetricsPanel', async () => {
        render(App);
        await state.init();

        // Initial SAMPLE_PREVIEW_MARKDOWN has 1 metric placeholder
        expect(state.detectedMetrics.length).toBe(1);
        const token = state.detectedMetrics[0];

        const metricInput = screen.getByPlaceholderText(
            'Enter verified metric (e.g. 42% latency reduction)'
        ) as HTMLInputElement;
        await fireEvent.input(metricInput, { target: { value: '48% latency reduction' } });

        const replaceBtn = screen.getByText('Replace in Resume');
        await fireEvent.click(replaceBtn);

        expect(state.artifacts.final).toContain('48% latency reduction');
        expect(state.artifacts.final).not.toContain(token);
        expect(state.detectedMetrics.length).toBe(0);
    });
});

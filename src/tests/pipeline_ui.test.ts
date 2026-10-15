import { beforeEach, describe, expect, it } from 'vitest';
import { mockIPC } from '@tauri-apps/api/mocks';
import { initStudio, state } from '../main';
import { loadStudioHtml } from './test_utils';

describe('Suite 9: Error Handling & Pipeline Precondition Guards', () => {
    beforeEach(() => {
        loadStudioHtml();

        mockIPC((cmd) => {
            if (cmd === 'get_vault_overview') {
                return {
                    root_path: 'C:/TestVault',
                    pii_files: [],
                    archetype_files: [],
                    style_files: [],
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

    it('TC-9.1: prevents Stage 1 Archetype Selection when Job Description is empty', async () => {
        await initStudio();

        const jdInput = document.getElementById('input-jd') as HTMLTextAreaElement;
        jdInput.value = '   '; // empty whitespace

        const stage1Btn = document.getElementById('btn-step-arch')!;
        stage1Btn.click();

        const statusDot = document.getElementById('agent-status-dot')!;
        const statusText = document.getElementById('agent-status-text')!;

        expect(statusDot.classList.contains('error')).toBe(true);
        expect(statusText.textContent).toContain('Please paste a Job Description first');
    });

    it('TC-9.2: prevents Stage 3 Parallel Checks when Draft and Final are empty', async () => {
        await initStudio();

        state.artifacts.draft = '';
        state.artifacts.final = '';

        const stage3Btn = document.getElementById('btn-step-checks')!;
        stage3Btn.click();

        const statusDot = document.getElementById('agent-status-dot')!;
        const statusText = document.getElementById('agent-status-text')!;

        expect(statusDot.classList.contains('error')).toBe(true);
        expect(statusText.textContent).toContain('Generate or paste a Resume Draft first');
    });

    it('TC-9.2: prevents Stage 4 Final Edit when Draft and Fact-Check are empty', async () => {
        await initStudio();

        state.artifacts.draft = '';
        state.artifacts.final = '';
        state.artifacts.fact_check = '';

        const stage4Btn = document.getElementById('btn-step-edit')!;
        stage4Btn.click();

        const statusDot = document.getElementById('agent-status-dot')!;
        const statusText = document.getElementById('agent-status-text')!;

        expect(statusDot.classList.contains('error')).toBe(true);
        expect(statusText.textContent).toContain(
            'Stage 4 Editor requires both a Draft Resume and a Fact-Check Audit report'
        );
    });
});

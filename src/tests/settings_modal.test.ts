import { beforeEach, describe, expect, it } from 'vitest';
import { mockIPC } from '@tauri-apps/api/mocks';
import { initStudio, state } from '../main';
import { loadStudioHtml } from './test_utils';

describe('Suite 2: AI Provider Settings & Connectivity', () => {
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

    it('TC-2.1: opens and closes the Settings modal with defaults', async () => {
        await initStudio();

        const modal = document.getElementById('settings-modal')!;
        expect(modal.classList.contains('hidden')).toBe(true);

        const openBtn = document.getElementById('btn-open-settings')!;
        openBtn.click();
        expect(modal.classList.contains('hidden')).toBe(false);

        const platformSelect = document.getElementById('cfg-platform') as HTMLSelectElement;
        expect(platformSelect.value).toBe('ollama');

        const closeBtn = document.getElementById('btn-close-settings')!;
        closeBtn.click();
        expect(modal.classList.contains('hidden')).toBe(true);
    });

    it('TC-2.2: configures Ollama provider, persists to localStorage, and updates topbar pill', async () => {
        await initStudio();

        const openBtn = document.getElementById('btn-open-settings')!;
        openBtn.click();

        const platformSelect = document.getElementById('cfg-platform') as HTMLSelectElement;
        platformSelect.value = 'ollama';

        const modelInput = document.getElementById('cfg-ollama-model') as HTMLInputElement;
        modelInput.value = 'llama3.1:8b';

        const saveBtn = document.getElementById('btn-save-settings')!;
        saveBtn.click();

        const modal = document.getElementById('settings-modal')!;
        expect(modal.classList.contains('hidden')).toBe(true);

        const pill = document.getElementById('active-provider-pill')!;
        expect(pill.textContent).toBe('Ollama • llama3.1:8b');

        const savedRaw = localStorage.getItem('resumai_llm_config');
        expect(savedRaw).not.toBeNull();
        const saved = JSON.parse(savedRaw!);
        expect(saved.platform).toBe('ollama');
        expect(saved.ollama_model).toBe('llama3.1:8b');
        expect(state.llmConfig.ollama_model).toBe('llama3.1:8b');
    });

    it('TC-2.3: configures Google Gemini API platform and updates pill', async () => {
        await initStudio();

        const openBtn = document.getElementById('btn-open-settings')!;
        openBtn.click();

        const platformSelect = document.getElementById('cfg-platform') as HTMLSelectElement;
        platformSelect.value = 'gemini';

        const keyInput = document.getElementById('cfg-gemini-key') as HTMLInputElement;
        keyInput.value = 'AIzaSyTestSecretKey_123456';

        const saveBtn = document.getElementById('btn-save-settings')!;
        saveBtn.click();

        const pill = document.getElementById('active-provider-pill')!;
        expect(pill.textContent).toBe('Gemini • gemini-3.5-flash');

        const saved = JSON.parse(localStorage.getItem('resumai_llm_config')!);
        expect(saved.platform).toBe('gemini');
        expect(saved.gemini_api_key).toBe('AIzaSyTestSecretKey_123456');
    });
});

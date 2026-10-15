import { afterEach, beforeEach } from 'vitest';
import { clearMocks, mockIPC } from '@tauri-apps/api/mocks';

// Flag to tell main.ts not to auto-invoke initStudio on DOMContentLoaded
(window as any).__TAURI_TEST_ENV__ = true;

beforeEach(() => {
    mockIPC((cmd) => {
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
        if (cmd === 'get_vault_overview') {
            return {
                root_path: 'C:/Vault',
                pii_files: [],
                archetype_files: [],
                style_files: [],
                prompt_files: [],
                applications: []
            };
        }
        if (cmd === 'read_style_css') {
            return 'body { font-family: sans-serif; }';
        }
    });
});

afterEach(() => {
    clearMocks();
    localStorage.clear();
});

import { beforeEach, describe, expect, it } from 'vitest';
import { mockIPC } from '@tauri-apps/api/mocks';
import { initStudio, state, switchStudioMode } from '../main';
import { loadStudioHtml } from './test_utils';

describe('Suite 1 & Suite 8: Career Vault & Workspace UI Management', () => {
    let writtenFiles: Record<string, string> = {};

    beforeEach(() => {
        writtenFiles = {};
        loadStudioHtml();

        mockIPC((cmd, payload: any) => {
            if (cmd === 'get_vault_overview') {
                return {
                    root_path: 'C:/TestVault',
                    pii_files: [
                        {
                            category: 'personal_info',
                            active_filename: 'personal_info.sample.md',
                            custom_filename: 'personal_info.md',
                            sample_filename: 'personal_info.sample.md',
                            is_custom: false
                        },
                        {
                            category: 'work_history',
                            active_filename: 'work_history.md',
                            custom_filename: 'work_history.md',
                            sample_filename: 'work_history.sample.md',
                            is_custom: true
                        }
                    ],
                    archetype_files: [
                        {
                            filename: 'staff_backend.sample.md',
                            is_sample: true,
                            is_active_in_selector: true
                        }
                    ],
                    style_files: ['resume.sample.css'],
                    prompt_files: ['resume_drafter.md'],
                    applications: [
                        {
                            slug: '20261015-acme-architect',
                            title: 'Acme Systems — Principal Architect',
                            created_at: '2026-10-15',
                            archetype_filename: 'staff_backend.sample.md',
                            match_score: 92,
                            status: 'completed',
                            has_final_resume: true,
                            has_cover_letter: false
                        }
                    ]
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
            if (cmd === 'read_vault_file') {
                return '# Sample PII Data\nName: Dr. Hank McCoy';
            }
            if (cmd === 'write_vault_file') {
                writtenFiles[`${payload.category}/${payload.filename}`] = payload.content;
                return undefined;
            }
            if (cmd === 'cmd_load_application') {
                return {
                    summary: {
                        slug: '20261015-acme-architect',
                        title: 'Acme Systems — Principal Architect',
                        created_at: '2026-10-15',
                        archetype_filename: 'staff_backend.sample.md',
                        match_score: 92,
                        status: 'completed',
                        has_final_resume: true,
                        has_cover_letter: false
                    },
                    job_description: 'Staff distributed systems role',
                    draft_markdown: '# Draft',
                    fact_check_markdown: '# Fact Check',
                    alignment_check_markdown: '# Alignment',
                    final_resume_markdown: '# Restored Final Resume',
                    cover_letter_markdown: '# Restored Cover Letter'
                };
            }
        });
    });

    it('TC-1.1: renders initial vault discovery with SAMPLE and CUSTOM badges', async () => {
        await initStudio();

        const piiList = document.getElementById('pii-list')!;
        const badges = piiList.querySelectorAll('.badge');
        expect(badges.length).toBe(2);
        expect(badges[0].textContent).toBe('SAMPLE');
        expect(badges[1].textContent).toBe('CUSTOM');

        const appsList = document.getElementById('applications-list')!;
        expect(appsList.children.length).toBe(1);
        expect(appsList.textContent).toContain('Acme Systems — Principal Architect');
        expect(appsList.textContent).toContain('92%');
    });

    it('TC-1.2: navigates between Pipeline Studio and Vault Markdown Editor', async () => {
        await initStudio();

        const pipelineView = document.getElementById('view-pipeline')!;
        const vaultView = document.getElementById('view-vault')!;

        expect(pipelineView.classList.contains('active')).toBe(true);
        expect(vaultView.classList.contains('active')).toBe(false);

        const tabVault = document.getElementById('tab-mode-vault')!;
        tabVault.click();

        expect(pipelineView.classList.contains('active')).toBe(false);
        expect(vaultView.classList.contains('active')).toBe(true);

        const tabPipeline = document.getElementById('tab-mode-pipeline')!;
        tabPipeline.click();

        expect(pipelineView.classList.contains('active')).toBe(true);
        expect(vaultView.classList.contains('active')).toBe(false);
    });

    it('TC-1.3 & TC-1.4: promotes sample vault file to custom override', async () => {
        await initStudio();

        switchStudioMode('vault');
        const editorTextarea = document.getElementById('vault-file-textarea') as HTMLTextAreaElement;
        editorTextarea.value = '# Custom Modified Content';

        const saveAsCustomBtn = document.getElementById('btn-promote-sample')!;
        saveAsCustomBtn.classList.remove('hidden');
        state.vaultEditor.category = 'pii';
        state.vaultEditor.filename = 'personal_info.sample.md';
        state.vaultEditor.customTargetFilename = 'personal_info.md';

        saveAsCustomBtn.click();

        // Wait for async write_vault_file
        await new Promise((r) => setTimeout(r, 50));

        expect(writtenFiles['pii/personal_info.md']).toBe('# Custom Modified Content');
    });

    it('TC-8.2 & TC-8.3: restores saved application workspace and clears on + New', async () => {
        await initStudio();

        const appsList = document.getElementById('applications-list')!;
        const appItem = appsList.querySelector('li') as HTMLElement;
        expect(appItem).not.toBeNull();

        appItem.click();
        await new Promise((r) => setTimeout(r, 100));

        const titleInput = document.getElementById('input-run-title') as HTMLInputElement;
        expect(titleInput.value).toBe('Acme Systems — Principal Architect');
        expect(state.artifacts.final).toBe('# Restored Final Resume');

        // Test TC-8.3: + New clears workspace
        const newBtn = document.getElementById('btn-new-application')!;
        newBtn.click();

        expect(titleInput.value).toBe('');
        const jdInput = document.getElementById('input-jd') as HTMLTextAreaElement;
        expect(jdInput.value).toBe('');
        expect(state.artifacts.draft).toBe('');
    });
});

import { invoke } from '@tauri-apps/api/core';
import { listen, type UnlistenFn } from '@tauri-apps/api/event';
import type {
    ArchetypeSelectionResult,
    ExportResult,
    FullPipelineResult,
    LlmConfig,
    LoadedApplicationWorkspace,
    ParallelChecksResult,
    StageArtifactResult,
    StreamEventPayload,
    VaultOverview
} from './types';

function isTauriRuntime(): boolean {
    return typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window;
}

export async function fetchVaultOverview(): Promise<VaultOverview> {
    if (!isTauriRuntime()) {
        return {
            root_path: 'Browser Preview Mode (Launch via `npm run tauri dev` for local filesystem I/O)',
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
                    active_filename: 'work_history.sample.md',
                    custom_filename: 'work_history.md',
                    sample_filename: 'work_history.sample.md',
                    is_custom: false
                }
            ],
            archetype_files: [
                {
                    filename: 'technical_consultant.sample.md',
                    is_sample: true,
                    is_active_in_selector: true
                },
                {
                    filename: 'staff_backend_engineer.sample.md',
                    is_sample: true,
                    is_active_in_selector: true
                }
            ],
            prompt_files: [
                'resume_archetype_selector.md',
                'resume_drafter.md',
                'resume_fact_checker.md',
                'resume_job_alignment_checker.md',
                'resume_editor.md',
                'resume_cover_letter.md'
            ],
            style_files: ['resume.sample.css'],
            applications: []
        };
    }
    return invoke<VaultOverview>('get_vault_overview');
}

export async function fetchDefaultLlmConfig(): Promise<LlmConfig> {
    if (!isTauriRuntime()) {
        return {
            platform: 'ollama',
            ollama_base_url: 'http://localhost:11434',
            ollama_model: 'qwen2.5:14b-instruct-q5_K_M',
            gemini_api_key: '',
            gemini_flash_model: 'gemini-3.5-flash',
            gemini_lite_model: 'gemini-3.5-flash-lite'
        };
    }
    return invoke<LlmConfig>('get_default_llm_config');
}

export async function readVaultFile(category: string, filename: string): Promise<string> {
    return invoke<string>('read_vault_file', { category, filename });
}

export async function writeVaultFile(category: string, filename: string, content: string): Promise<string> {
    return invoke<string>('write_vault_file', { category, filename, content });
}

export async function readStyleCss(filename?: string): Promise<string> {
    if (!isTauriRuntime()) {
        return 'body { font-family: Georgia, serif; padding: 0.4in; color: #111; } h1 { margin-bottom: 4px; } h2 { border-bottom: 1px solid #888; margin-top: 14px; font-size: 13pt; }';
    }
    return invoke<string>('read_style_css', { filename: filename ?? null });
}

export async function appendFactToWorkHistory(factBullet: string, roleContext?: string): Promise<string> {
    return invoke<string>('append_fact_to_work_history', {
        factBullet,
        roleContext: roleContext ?? null
    });
}

export async function runSelectArchetype(config: LlmConfig, jobDescription: string): Promise<ArchetypeSelectionResult> {
    return invoke<ArchetypeSelectionResult>('cmd_select_archetype', {
        config,
        jobDescription
    });
}

export async function runDraftResume(
    config: LlmConfig,
    jobDescription?: string,
    archetypeFilename?: string
): Promise<StageArtifactResult> {
    return invoke<StageArtifactResult>('cmd_draft_resume', {
        config,
        jobDescription: jobDescription || null,
        archetypeFilename: archetypeFilename || null
    });
}

export async function runParallelChecks(
    config: LlmConfig,
    resumeMarkdown: string,
    jobDescription?: string
): Promise<ParallelChecksResult> {
    return invoke<ParallelChecksResult>('cmd_run_parallel_checks', {
        config,
        resumeMarkdown,
        jobDescription: jobDescription || null
    });
}

export async function runEditResume(
    config: LlmConfig,
    resumeMarkdown: string,
    factCheckReport: string,
    jobAlignmentReport?: string,
    userNotes?: string
): Promise<StageArtifactResult> {
    return invoke<StageArtifactResult>('cmd_edit_resume', {
        config,
        resumeMarkdown,
        factCheckReport,
        jobAlignmentReport: jobAlignmentReport || null,
        userNotes: userNotes || null
    });
}

export async function runGenerateCoverLetter(
    config: LlmConfig,
    resumeMarkdown: string,
    jobDescription: string | undefined,
    archetypeFilename: string | undefined,
    mode: string
): Promise<StageArtifactResult> {
    return invoke<StageArtifactResult>('cmd_generate_cover_letter', {
        config,
        resumeMarkdown,
        jobDescription: jobDescription || null,
        archetypeFilename: archetypeFilename || null,
        mode
    });
}

export async function runFullPipeline(
    config: LlmConfig,
    runTitle?: string,
    jobDescription?: string,
    archetypeOverride?: string,
    styleFilename?: string,
    userNotes?: string
): Promise<FullPipelineResult> {
    return invoke<FullPipelineResult>('cmd_run_full_pipeline', {
        config,
        runTitle: runTitle || null,
        jobDescription: jobDescription || null,
        archetypeOverride: archetypeOverride || null,
        styleFilename: styleFilename || null,
        userNotes: userNotes || null
    });
}

export async function exportResumeBundle(
    markdown: string,
    styleFilename?: string,
    outputName?: string
): Promise<ExportResult> {
    return invoke<ExportResult>('cmd_export_resume', {
        markdown,
        styleFilename: styleFilename || null,
        outputName: outputName || null
    });
}

export async function loadApplicationWorkspace(slug: string): Promise<LoadedApplicationWorkspace> {
    return invoke<LoadedApplicationWorkspace>('cmd_load_application', { slug });
}

export async function runRefinePageFit(
    config: LlmConfig,
    resumeMarkdown: string,
    targetPages: number,
    deltaLines: number
): Promise<StageArtifactResult> {
    if (!isTauriRuntime()) {
        return {
            markdown: resumeMarkdown,
            thinking: 'Browser mock: Refinement simulated.',
            model_used: 'browser-mock',
            metric_placeholders: []
        };
    }
    return invoke<StageArtifactResult>('cmd_refine_page_fit', {
        config,
        resumeMarkdown,
        targetPages,
        deltaLines
    });
}

export async function subscribeAgentStream(handler: (payload: StreamEventPayload) => void): Promise<UnlistenFn> {
    if (!isTauriRuntime()) {
        return () => {};
    }
    return listen<StreamEventPayload>('agent:stream', (event) => {
        handler(event.payload);
    });
}

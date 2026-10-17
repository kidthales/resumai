export interface PiiEntry {
    category: string;
    active_filename: string;
    custom_filename: string;
    sample_filename: string;
    is_custom: boolean;
}

export interface ArchetypeEntry {
    filename: string;
    is_sample: boolean;
    is_active_in_selector: boolean;
}

export interface ApplicationRunSummary {
    slug: string;
    created_at: string;
    title: string;
    archetype_filename: string | null;
    match_score: number | null;
    status: string;
    has_final_resume: boolean;
    has_cover_letter: boolean;
}

export interface VaultOverview {
    root_path: string;
    pii_files: PiiEntry[];
    archetype_files: ArchetypeEntry[];
    prompt_files: string[];
    style_files: string[];
    applications: ApplicationRunSummary[];
}

export interface LlmConfig {
    platform: 'ollama' | 'gemini';
    ollama_base_url: string;
    ollama_model: string;
    gemini_api_key: string;
    gemini_flash_model: string;
    gemini_lite_model: string;
}

export interface ArchetypeSelectionResult {
    archetype_filename: string;
    rationale: string;
    thinking: string;
    model_used: string;
}

export interface StageArtifactResult {
    markdown: string;
    thinking: string;
    model_used: string;
    metric_placeholders: string[];
}

export interface AlignmentCheckResult {
    markdown: string;
    thinking: string;
    model_used: string;
    match_score: number | null;
}

export interface ParallelChecksResult {
    fact_check: StageArtifactResult;
    alignment_check: AlignmentCheckResult | null;
}

export interface FullPipelineResult {
    slug: string;
    output_dir: string;
    archetype: ArchetypeSelectionResult | null;
    draft: StageArtifactResult;
    fact_check: StageArtifactResult;
    alignment_check: AlignmentCheckResult | null;
    final_resume: StageArtifactResult;
    exported_html_path: string;
    exported_docx_path: string;
}

export interface LoadedApplicationWorkspace {
    summary: ApplicationRunSummary;
    job_description: string;
    archetype_selection: ArchetypeSelectionResult | null;
    draft_markdown: string;
    fact_check_markdown: string;
    alignment_check_markdown: string;
    final_resume_markdown: string;
    cover_letter_markdown: string;
}

export interface ExportResult {
    html_path: string;
    docx_path: string;
}

export interface StreamEventPayload {
    stage: string;
    kind: 'thinking' | 'text' | 'status' | 'tool';
    chunk: string;
}

export type ArtifactTabKey = 'final' | 'draft' | 'fact_check' | 'alignment' | 'cover_letter';

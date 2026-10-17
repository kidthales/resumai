import { flushSync } from 'svelte';
import {
    appendFactToWorkHistory,
    exportResumeBundle,
    fetchDefaultLlmConfig,
    fetchVaultOverview,
    loadApplicationWorkspace,
    readStyleCss,
    readVaultFile,
    runDraftResume,
    runEditResume,
    runFullPipeline,
    runGenerateCoverLetter,
    runParallelChecks,
    runRefinePageFit,
    runSelectArchetype,
    subscribeAgentStream,
    writeVaultFile
} from '../api';
import type { ArtifactTabKey, LlmConfig, VaultOverview } from '../types';
import { detectMetrics, replaceMetric } from './metrics';
import { calculatePageBudget } from './pageBudget';
import { SAMPLE_PREVIEW_MARKDOWN } from './state';

const DEFAULT_THONK_PLACEHOLDER = 'Agent reasoning (.thonk) and tool calls will stream here in real time...';

export class StudioStore {
    overview = $state<VaultOverview | null>(null);
    llmConfig = $state<LlmConfig>({
        platform: 'ollama',
        ollama_base_url: 'http://localhost:11434',
        ollama_model: 'qwen2.5:14b-instruct-q5_K_M',
        gemini_api_key: '',
        gemini_flash_model: 'gemini-3.5-flash',
        gemini_lite_model: 'gemini-3.5-flash-lite'
    });
    activeMode = $state<'pipeline' | 'vault'>('pipeline');
    activeArtifactTab = $state<ArtifactTabKey>('final');
    artifacts = $state<Record<ArtifactTabKey, string>>({
        final: SAMPLE_PREVIEW_MARKDOWN,
        draft: '',
        fact_check: '',
        alignment: '',
        cover_letter: ''
    });
    matchScore = $state<number | null>(null);
    selectedArchetype = $state('');
    archetypeRationale = $state('');
    activeStyleFile = $state('resume.sample.css');
    activeCssContent = $state('');
    fontSizePt = $state(10);
    lineHeight = $state(1.35);
    marginIn = $state(0.45);
    targetPages = $state<1 | 2>(2);
    vaultEditor = $state<{
        category: string;
        filename: string;
        customTargetFilename: string | null;
    }>({
        category: 'pii',
        filename: 'work_history.sample.md',
        customTargetFilename: 'work_history.md'
    });
    isRunning = $state(false);

    // Form & Interactive Studio Inputs
    runTitle = $state('');
    jobDescription = $state('');
    userNotes = $state('');
    pitchMode = $state<'cover_letter' | 'consulting_pitch'>('cover_letter');
    vaultFileContent = $state('');
    vaultEditorHint = $state('');
    agentStatus = $state<'idle' | 'running' | 'error'>('idle');
    agentStatusText = $state('Ready — Select a stage or Run Full Pipeline');
    thonkLog = $state(DEFAULT_THONK_PLACEHOLDER);
    thonkDrawerOpen = $state(true);
    exportBannerText = $state('');
    settingsModalOpen = $state(false);
    sidebarWidthPx = $state(260);
    previewWidthPx = $state(520);
    iframeScrollHeight = $state(1056);

    // Derived computations
    providerPillText = $derived(
        this.llmConfig.platform === 'gemini'
            ? `Gemini • ${this.llmConfig.gemini_flash_model}`
            : `Ollama • ${this.llmConfig.ollama_model}`
    );

    previewMarkdown = $derived(
        this.artifacts[this.activeArtifactTab] ||
            this.artifacts.final ||
            this.artifacts.draft ||
            SAMPLE_PREVIEW_MARKDOWN
    );

    detectedMetrics = $derived(
        detectMetrics(this.artifacts[this.activeArtifactTab] || this.artifacts.final || this.artifacts.draft)
    );

    pageBudget = $derived(calculatePageBudget(this.iframeScrollHeight, this.targetPages));

    reset(): void {
        this.overview = null;
        this.llmConfig = {
            platform: 'ollama',
            ollama_base_url: 'http://localhost:11434',
            ollama_model: 'qwen2.5:14b-instruct-q5_K_M',
            gemini_api_key: '',
            gemini_flash_model: 'gemini-3.5-flash',
            gemini_lite_model: 'gemini-3.5-flash-lite'
        };
        this.activeMode = 'pipeline';
        this.activeArtifactTab = 'final';
        this.artifacts = {
            final: SAMPLE_PREVIEW_MARKDOWN,
            draft: '',
            fact_check: '',
            alignment: '',
            cover_letter: ''
        };
        this.matchScore = null;
        this.selectedArchetype = '';
        this.archetypeRationale = '';
        this.activeStyleFile = 'resume.sample.css';
        this.activeCssContent = '';
        this.fontSizePt = 10;
        this.lineHeight = 1.35;
        this.marginIn = 0.45;
        this.targetPages = 2;
        this.vaultEditor = {
            category: 'pii',
            filename: 'work_history.sample.md',
            customTargetFilename: 'work_history.md'
        };
        this.isRunning = false;
        this.runTitle = '';
        this.jobDescription = '';
        this.userNotes = '';
        this.pitchMode = 'cover_letter';
        this.vaultFileContent = '';
        this.vaultEditorHint = '';
        this.agentStatus = 'idle';
        this.agentStatusText = 'Ready — Select a stage or Run Full Pipeline';
        this.thonkLog = DEFAULT_THONK_PLACEHOLDER;
        this.thonkDrawerOpen = true;
        this.exportBannerText = '';
        this.settingsModalOpen = false;
        this.iframeScrollHeight = 1056;
    }

    setStatus(status: 'idle' | 'running' | 'error', message: string): void {
        flushSync(() => {
            this.agentStatus = status;
            this.agentStatusText = message;
        });
    }

    appendThonkLog(line: string, inline = false): void {
        if (this.thonkLog.startsWith('Agent reasoning (.thonk)')) {
            this.thonkLog = '';
        }
        this.thonkLog += inline ? line : `\n${line}`;
    }

    clearThonkLog(header: string): void {
        this.thonkLog = `=== ${header} ===\n`;
    }

    switchStudioMode(mode: 'pipeline' | 'vault'): void {
        flushSync(() => {
            this.activeMode = mode;
        });
    }

    switchArtifactTab(tab: ArtifactTabKey): void {
        flushSync(() => {
            this.activeArtifactTab = tab;
        });
    }

    replaceMetricInActiveTab(token: string, val: string): void {
        const trimmed = val.trim();
        if (!trimmed) return;
        flushSync(() => {
            const targetTab = this.activeArtifactTab;
            this.artifacts[targetTab] = replaceMetric(this.artifacts[targetTab], token, trimmed);
        });
    }

    async saveMetricToWorkHistory(token: string, val: string): Promise<void> {
        const trimmed = val.trim();
        if (!trimmed) return;
        const roleTitle = this.runTitle.trim() || 'Verified Metric Update';
        try {
            await appendFactToWorkHistory(trimmed, roleTitle);
            this.artifacts[this.activeArtifactTab] = replaceMetric(
                this.artifacts[this.activeArtifactTab],
                token,
                trimmed
            );
            await this.refreshVaultOverview();
            this.setStatus('idle', 'Saved verified metric to content/pii/work_history.md!');
        } catch (err) {
            this.setStatus('error', `Failed to save metric: ${String(err)}`);
        }
    }

    async openVaultFileInEditor(
        category: string,
        filename: string,
        customTargetFilename: string | null = null
    ): Promise<void> {
        try {
            const content = await readVaultFile(category, filename);
            this.vaultEditor = { category, filename, customTargetFilename };
            if (customTargetFilename && filename.endsWith('.sample.md')) {
                this.vaultEditorHint =
                    'Viewing sample template — click Save as Custom to create your personal git-ignored file.';
            } else {
                this.vaultEditorHint = 'Changes take effect immediately on the next agent run.';
            }
            this.vaultFileContent = content;
            this.switchStudioMode('vault');
        } catch (err) {
            this.setStatus('error', `Failed to read ${category}/${filename}: ${String(err)}`);
        }
    }

    async refreshVaultOverview(): Promise<void> {
        const overview = await fetchVaultOverview();
        this.overview = overview;
        if (overview.style_files.length > 0 && !overview.style_files.includes(this.activeStyleFile)) {
            this.activeStyleFile = overview.style_files[0];
        }
    }

    async loadSavedApplication(slug: string): Promise<void> {
        try {
            const ws = await loadApplicationWorkspace(slug);
            this.runTitle = ws.summary.title;
            this.jobDescription = ws.job_description;
            this.artifacts.draft = ws.draft_markdown;
            this.artifacts.fact_check = ws.fact_check_markdown;
            this.artifacts.alignment = ws.alignment_check_markdown;
            this.artifacts.final = ws.final_resume_markdown || ws.draft_markdown;
            this.artifacts.cover_letter = ws.cover_letter_markdown;

            if (ws.archetype_selection) {
                this.selectedArchetype = ws.archetype_selection.archetype_filename;
                this.archetypeRationale = `${ws.archetype_selection.archetype_filename}: ${ws.archetype_selection.rationale}`;
            } else {
                this.archetypeRationale = '';
            }

            this.matchScore = ws.summary.match_score;
            this.switchStudioMode('pipeline');
            this.switchArtifactTab('final');
            this.setStatus('idle', `Loaded application workspace: ${ws.summary.slug}`);
        } catch (err) {
            this.setStatus('error', `Failed to load application: ${String(err)}`);
        }
    }

    startNewApplication(): void {
        this.runTitle = '';
        this.jobDescription = '';
        this.selectedArchetype = '';
        this.archetypeRationale = '';
        this.matchScore = null;
        this.artifacts = {
            final: '',
            draft: '',
            fact_check: '',
            alignment: '',
            cover_letter: ''
        };
        this.switchStudioMode('pipeline');
        this.switchArtifactTab('draft');
        this.setStatus('idle', 'Started clean application workspace.');
    }

    async startNewArchetype(): Promise<void> {
        const sample = await readVaultFile('archetypes', 'technical_consultant.sample.md').catch(
            () => '# Custom Archetype\n\n## Profile Overview\n'
        );
        this.vaultEditor = {
            category: 'archetypes',
            filename: 'custom_archetype.md',
            customTargetFilename: null
        };
        this.vaultEditorHint = 'Creating new custom archetype content/archetypes/custom_archetype.md';
        this.vaultFileContent = sample;
        this.switchStudioMode('vault');
    }

    async startNewStyle(): Promise<void> {
        const sampleCss = await readStyleCss('resume.sample.css');
        this.vaultEditor = {
            category: 'styles',
            filename: 'resume.custom.css',
            customTargetFilename: null
        };
        this.vaultEditorHint = 'Creating custom stylesheet assets/styles/resumes/resume.custom.css';
        this.vaultFileContent = sampleCss;
        this.switchStudioMode('vault');
    }

    async saveCurrentVaultFile(): Promise<void> {
        const { category, filename } = this.vaultEditor;
        try {
            const savedPath = await writeVaultFile(category, filename, this.vaultFileContent);
            await this.refreshVaultOverview();
            if (category === 'styles' && filename === this.activeStyleFile) {
                this.activeCssContent = this.vaultFileContent;
            }
            this.setStatus('idle', `Saved ${savedPath}`);
        } catch (err) {
            this.setStatus('error', `Failed to save file: ${String(err)}`);
        }
    }

    async promoteSampleVaultFile(): Promise<void> {
        const { category, customTargetFilename } = this.vaultEditor;
        if (!customTargetFilename) return;
        try {
            const savedPath = await writeVaultFile(category, customTargetFilename, this.vaultFileContent);
            this.vaultEditor.filename = customTargetFilename;
            this.vaultEditor.customTargetFilename = null;
            await this.refreshVaultOverview();
            this.setStatus('idle', `Created custom override: ${savedPath}`);
        } catch (err) {
            this.setStatus('error', `Failed to create custom override: ${String(err)}`);
        }
    }

    async executeStage1Archetype(): Promise<void> {
        const jd = this.jobDescription.trim();
        if (!jd) {
            this.setStatus('error', 'Please paste a Job Description first to run Archetype Selection.');
            return;
        }
        this.isRunning = true;
        this.clearThonkLog('Stage 1: Resume Archetype Selector');
        this.setStatus('running', 'Stage 1: Selecting optimal Candidate Archetype via tool calls...');
        try {
            const res = await runSelectArchetype(this.llmConfig, jd);
            this.selectedArchetype = res.archetype_filename;
            this.archetypeRationale = `${res.archetype_filename}: ${res.rationale}`;
            this.setStatus('idle', `Selected Archetype: ${res.archetype_filename} (${res.model_used})`);
        } catch (err) {
            this.setStatus('error', `Archetype Selector failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    async executeStage2Draft(): Promise<void> {
        const jd = this.jobDescription.trim();
        const arch = this.selectedArchetype.trim();
        this.isRunning = true;
        this.artifacts.draft = '';
        this.switchArtifactTab('draft');
        this.clearThonkLog('Stage 2: Resume Drafter');
        this.setStatus('running', 'Stage 2: Synthesizing initial Markdown resume draft...');
        try {
            const res = await runDraftResume(this.llmConfig, jd || undefined, arch || undefined);
            this.artifacts.draft = res.markdown;
            this.setStatus('idle', `Draft complete (${res.model_used}). Ready for Stage 3 Parallel Checks.`);
        } catch (err) {
            this.setStatus('error', `Resume Drafter failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    async executeStage3ParallelChecks(): Promise<void> {
        const baseline = this.artifacts.draft.trim() || this.artifacts.final.trim();
        if (!baseline) {
            this.setStatus('error', 'Generate or paste a Resume Draft first before running checks.');
            return;
        }
        const jd = this.jobDescription.trim();
        this.isRunning = true;
        this.artifacts.fact_check = '';
        this.artifacts.alignment = '';
        this.switchArtifactTab('fact_check');
        this.clearThonkLog('Stage 3a & 3b: Parallel Fact-Check & Job-Alignment Check (tokio::join!)');
        this.setStatus('running', 'Stage 3: Running Fact-Checker and Job-Alignment Checker concurrently...');
        try {
            const res = await runParallelChecks(this.llmConfig, baseline, jd || undefined);
            this.artifacts.fact_check = res.fact_check.markdown;
            if (res.alignment_check) {
                this.artifacts.alignment = res.alignment_check.markdown;
                this.matchScore = res.alignment_check.match_score;
            }
            this.setStatus('idle', 'Parallel Fact-Check & Alignment Audit complete! Review tabs or run Stage 4.');
        } catch (err) {
            this.setStatus('error', `Parallel checks failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    async executeStage4Editor(): Promise<void> {
        const baseline = this.artifacts.draft.trim() || this.artifacts.final.trim();
        const fc = this.artifacts.fact_check.trim();
        if (!baseline || !fc) {
            this.setStatus('error', 'Stage 4 Editor requires both a Draft Resume and a Fact-Check Audit report.');
            return;
        }
        const jac = this.artifacts.alignment.trim();
        const notes = this.userNotes.trim();
        this.isRunning = true;
        this.artifacts.final = '';
        this.switchArtifactTab('final');
        this.clearThonkLog('Stage 4: Resume Editor Synthesis');
        this.setStatus('running', 'Stage 4: Applying Fact-Check & Job-Alignment revisions...');
        try {
            const res = await runEditResume(this.llmConfig, baseline, fc, jac || undefined, notes || undefined);
            this.artifacts.final = res.markdown;
            this.setStatus('idle', `Final Resume synthesized (${res.model_used})!`);
        } catch (err) {
            this.setStatus('error', `Resume Editor failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    async executeStage5Pitch(): Promise<void> {
        const resumeMd = this.artifacts.final.trim() || this.artifacts.draft.trim();
        if (!resumeMd) {
            this.setStatus('error', 'Please generate a draft or final resume first.');
            return;
        }
        const jd = this.jobDescription.trim();
        const arch = this.selectedArchetype.trim();
        const mode = this.pitchMode;
        this.isRunning = true;
        this.artifacts.cover_letter = '';
        this.switchArtifactTab('cover_letter');
        this.clearThonkLog(
            `Stage 5: ${mode === 'consulting_pitch' ? 'Consulting Proposal' : 'Executive Cover Letter'}`
        );
        this.setStatus('running', 'Stage 5: Synthesizing grounded Cover Letter / Consulting Pitch...');
        try {
            const res = await runGenerateCoverLetter(
                this.llmConfig,
                resumeMd,
                jd || undefined,
                arch || undefined,
                mode
            );
            this.artifacts.cover_letter = res.markdown;
            this.setStatus('idle', `Generated ${mode === 'consulting_pitch' ? 'Consulting Pitch' : 'Cover Letter'}!`);
        } catch (err) {
            this.setStatus('error', `Cover Letter / Pitch generation failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    async executeFullPipeline(): Promise<void> {
        const title = this.runTitle.trim();
        const jd = this.jobDescription.trim();
        const archOverride = this.selectedArchetype.trim();
        const notes = this.userNotes.trim();

        this.isRunning = true;
        this.artifacts = {
            final: '',
            draft: '',
            fact_check: '',
            alignment: '',
            cover_letter: this.artifacts.cover_letter
        };
        this.switchArtifactTab('draft');
        this.clearThonkLog('Full End-to-End Pipeline (Archetype -> Draft -> Parallel Checks -> Edit -> Export)');
        this.setStatus('running', 'Running Full Pipeline (Parallel Checks + Auto-Export)...');

        try {
            const res = await runFullPipeline(
                this.llmConfig,
                title || undefined,
                jd || undefined,
                archOverride || undefined,
                this.activeStyleFile,
                notes || undefined
            );

            if (res.archetype) {
                this.selectedArchetype = res.archetype.archetype_filename;
                this.archetypeRationale = `${res.archetype.archetype_filename}: ${res.archetype.rationale}`;
            }

            this.artifacts.draft = res.draft.markdown;
            this.artifacts.fact_check = res.fact_check.markdown;
            if (res.alignment_check) {
                this.artifacts.alignment = res.alignment_check.markdown;
                this.matchScore = res.alignment_check.match_score;
            }
            this.artifacts.final = res.final_resume.markdown;

            this.switchArtifactTab('final');
            await this.refreshVaultOverview();

            this.exportBannerText = `Saved workspace & exported: ${res.exported_docx_path}`;
            this.setStatus('idle', `Full pipeline complete! Workspace saved to var/applications/${res.slug}`);
        } catch (err) {
            this.setStatus('error', `Full pipeline failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    async changeStyleFile(newStyleFile: string): Promise<void> {
        this.activeStyleFile = newStyleFile;
        this.activeCssContent = await readStyleCss(this.activeStyleFile);
    }

    async exportBundle(): Promise<void> {
        const md = this.artifacts[this.activeArtifactTab] || this.artifacts.final || this.artifacts.draft;
        if (!md.trim()) return;
        const title = this.runTitle.trim();
        const stem = title
            ? title
                  .toLowerCase()
                  .replace(/[^a-z0-9]+/g, '-')
                  .replace(/^-|-$/g, '')
            : 'resume_export';
        try {
            const res = await exportResumeBundle(md, this.activeStyleFile, stem);
            this.exportBannerText = `Exported DOCX & HTML -> ${res.docx_path}`;
            this.setStatus('idle', `Exported ${res.docx_path} and ${res.html_path}`);
        } catch (err) {
            this.setStatus('error', `Export failed: ${String(err)}`);
        }
    }

    async autoFitPageBudget(): Promise<void> {
        if (this.isRunning) return;
        const resumeMd = this.artifacts.final || this.artifacts[this.activeArtifactTab];
        if (!resumeMd.trim()) {
            this.setStatus('error', 'No resume markdown available to optimize for page budget.');
            return;
        }

        const budget = this.pageBudget;
        if (Math.abs(budget.excessPx) <= 15) {
            this.setStatus(
                'idle',
                `Resume is already optimally fitted to ${this.targetPages}.00 page(s) (${budget.pagesFormatted} pages)!`
            );
            return;
        }

        this.isRunning = true;
        const actionVerb =
            budget.deltaLines > 0
                ? `trimming ~${budget.deltaLines} lines`
                : `expanding ~${Math.abs(budget.deltaLines)} lines`;
        this.setStatus('running', `📐 Optimizing page budget: ${actionVerb} to fit ${this.targetPages}.00 page(s)...`);

        this.thonkDrawerOpen = true;
        this.thonkLog = `[PageFitOptimizer] Initiating refinement pass targeting ${this.targetPages}.00 page(s) (Delta: ${budget.deltaLines} lines, ${budget.excessPx}px)...\n`;

        try {
            const result = await runRefinePageFit(this.llmConfig, resumeMd, this.targetPages, budget.deltaLines);
            this.artifacts.final = result.markdown;
            this.setStatus('idle', `✨ Page budget optimized to fit ${this.targetPages}.00 page(s)!`);
        } catch (err) {
            this.setStatus('error', `Page-fit optimization failed: ${String(err)}`);
        } finally {
            this.isRunning = false;
        }
    }

    saveSettings(nextConfig: LlmConfig): void {
        this.llmConfig = {
            platform: nextConfig.platform,
            ollama_base_url: nextConfig.ollama_base_url.trim() || 'http://localhost:11434',
            ollama_model: nextConfig.ollama_model.trim() || 'qwen2.5:14b-instruct-q5_K_M',
            gemini_api_key: nextConfig.gemini_api_key.trim(),
            gemini_flash_model: nextConfig.gemini_flash_model.trim() || 'gemini-3.5-flash',
            gemini_lite_model: nextConfig.gemini_lite_model.trim() || 'gemini-3.5-flash-lite'
        };
        localStorage.setItem('resumai_llm_config', JSON.stringify(this.llmConfig));
        this.settingsModalOpen = false;
        this.setStatus('idle', `Saved AI Provider Settings (${this.llmConfig.platform})`);
    }

    async init(): Promise<void> {
        this.llmConfig = await fetchDefaultLlmConfig();
        const savedCfg = localStorage.getItem('resumai_llm_config');
        if (savedCfg) {
            try {
                this.llmConfig = { ...this.llmConfig, ...JSON.parse(savedCfg) };
            } catch {
                // Ignore corrupt localStorage
            }
        }

        const savedLayout = localStorage.getItem('resumai_pane_widths');
        if (savedLayout) {
            try {
                const parsed = JSON.parse(savedLayout);
                if (typeof parsed.sidebar === 'number') this.sidebarWidthPx = parsed.sidebar;
                if (typeof parsed.preview === 'number') this.previewWidthPx = parsed.preview;
            } catch {
                // Ignore corrupt localStorage
            }
        }

        await this.refreshVaultOverview();
        this.activeCssContent = await readStyleCss(this.activeStyleFile);

        await subscribeAgentStream((payload) => {
            if (payload.kind === 'thinking') {
                this.appendThonkLog(payload.chunk, true);
            } else if (payload.kind === 'tool' || payload.kind === 'status') {
                this.appendThonkLog(`\n[${payload.stage.toUpperCase()}] ${payload.chunk}\n`, false);
            } else if (payload.kind === 'text') {
                const stageToTab: Record<string, ArtifactTabKey> = {
                    resume_drafter: 'draft',
                    fact_check: 'fact_check',
                    fact_checker: 'fact_check',
                    job_alignment_checker: 'alignment',
                    resume_editor: 'final',
                    cover_letter: 'cover_letter'
                };
                const targetTab = stageToTab[payload.stage];
                if (targetTab) {
                    this.artifacts[targetTab] += payload.chunk;
                }
            }
        });
    }
}

export const state = new StudioStore();

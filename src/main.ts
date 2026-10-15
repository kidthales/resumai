import { marked } from 'marked';
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
    runSelectArchetype,
    subscribeAgentStream,
    writeVaultFile
} from './api';
import type { ArtifactTabKey } from './types';
import { detectMetrics, replaceMetric } from './lib/metrics';
import { calculatePageBudget } from './lib/pageBudget';
import { SAMPLE_PREVIEW_MARKDOWN, StudioState, createDefaultState } from './lib/state';

export const state: StudioState = createDefaultState();

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) {
        throw new Error(`Missing DOM element #${id}`);
    }
    return node as T;
}

function setStatus(status: 'idle' | 'running' | 'error', message: string): void {
    const dot = el<HTMLSpanElement>('agent-status-dot');
    const text = el<HTMLElement>('agent-status-text');
    dot.className = `status-dot ${status}`;
    text.textContent = message;
}

function appendThonkLog(line: string, inline = false): void {
    const pre = el<HTMLPreElement>('thonk-stream-log');
    if (pre.textContent?.startsWith('Agent reasoning (.thonk)')) {
        pre.textContent = '';
    }
    pre.textContent += inline ? line : `\n${line}`;
    pre.scrollTop = pre.scrollHeight;
}

function clearThonkLog(header: string): void {
    const pre = el<HTMLPreElement>('thonk-stream-log');
    pre.textContent = `=== ${header} ===\n`;
}

function updateProviderPill(): void {
    const pill = el<HTMLSpanElement>('active-provider-pill');
    if (state.llmConfig.platform === 'gemini') {
        pill.textContent = `Gemini • ${state.llmConfig.gemini_flash_model}`;
    } else {
        pill.textContent = `Ollama • ${state.llmConfig.ollama_model}`;
    }
}

function switchStudioMode(mode: 'pipeline' | 'vault'): void {
    state.activeMode = mode;
    el('tab-mode-pipeline').classList.toggle('active', mode === 'pipeline');
    el('tab-mode-vault').classList.toggle('active', mode === 'vault');
    el('view-pipeline').classList.toggle('active', mode === 'pipeline');
    el('view-vault').classList.toggle('active', mode === 'vault');
}

function switchArtifactTab(tab: ArtifactTabKey): void {
    const editor = el<HTMLTextAreaElement>('artifact-editor');
    if (state.activeArtifactTab !== tab) {
        state.artifacts[state.activeArtifactTab] = editor.value;
    }

    state.activeArtifactTab = tab;
    document.querySelectorAll<HTMLButtonElement>('.artifact-tab').forEach((btn) => {
        btn.classList.toggle('active', btn.dataset.artifact === tab);
    });

    editor.value = state.artifacts[tab] || '';
    renderLivePreview();
    renderMetricPlaceholders();
}

function renderMetricPlaceholders(): void {
    const panel = el<HTMLDivElement>('metrics-panel');
    const listEl = el<HTMLDivElement>('metrics-list');
    const activeMd = state.artifacts[state.activeArtifactTab] || state.artifacts.final || state.artifacts.draft;
    const placeholders = detectMetrics(activeMd);

    if (placeholders.length === 0) {
        panel.classList.add('hidden');
        listEl.innerHTML = '';
        return;
    }

    panel.classList.remove('hidden');
    listEl.innerHTML = '';

    placeholders.forEach((token) => {
        const row = document.createElement('div');
        row.className = 'metric-row';

        const code = document.createElement('span');
        code.className = 'metric-token';
        code.textContent = token;

        const input = document.createElement('input');
        input.type = 'text';
        input.className = 'flex-1';
        input.placeholder = 'Enter verified metric (e.g. 42% latency reduction)';

        const btnReplace = document.createElement('button');
        btnReplace.type = 'button';
        btnReplace.className = 'btn btn-secondary btn-sm';
        btnReplace.textContent = 'Replace in Resume';
        btnReplace.addEventListener('click', () => {
            const val = input.value.trim();
            if (!val) return;
            const targetTab = state.activeArtifactTab;
            state.artifacts[targetTab] = replaceMetric(state.artifacts[targetTab], token, val);
            el<HTMLTextAreaElement>('artifact-editor').value = state.artifacts[targetTab];
            renderLivePreview();
            renderMetricPlaceholders();
        });

        const btnWriteBack = document.createElement('button');
        btnWriteBack.type = 'button';
        btnWriteBack.className = 'btn btn-primary btn-sm';
        btnWriteBack.textContent = '💾 Save Fact to work_history.md';
        btnWriteBack.addEventListener('click', async () => {
            const val = input.value.trim();
            if (!val) return;
            const roleTitle = el<HTMLInputElement>('input-run-title').value.trim();
            try {
                await appendFactToWorkHistory(val, roleTitle || 'Verified Metric Update');
                state.artifacts[state.activeArtifactTab] = replaceMetric(
                    state.artifacts[state.activeArtifactTab],
                    token,
                    val
                );
                el<HTMLTextAreaElement>('artifact-editor').value = state.artifacts[state.activeArtifactTab];
                renderLivePreview();
                renderMetricPlaceholders();
                await refreshVaultOverview();
                setStatus('idle', 'Saved verified metric to content/pii/work_history.md!');
            } catch (err) {
                setStatus('error', `Failed to save metric: ${String(err)}`);
            }
        });

        row.append(code, input, btnReplace, btnWriteBack);
        listEl.appendChild(row);
    });
}

function renderLivePreview(): void {
    const iframe = el<HTMLIFrameElement>('preview-iframe');
    const mdToRender =
        state.artifacts[state.activeArtifactTab] ||
        state.artifacts.final ||
        state.artifacts.draft ||
        SAMPLE_PREVIEW_MARKDOWN;

    const renderedHtml = marked.parse(mdToRender, { async: false }) as string;

    // Visual 11-inch page-break indicator background in screen mode (hidden during @media print)
    const overrideCss = `
    ${state.activeCssContent}
    html, body {
      font-size: ${state.fontSizePt}pt !important;
      line-height: ${state.lineHeight} !important;
    }
    @media screen {
      body {
        padding: ${state.marginIn}in !important;
        max-width: 8.5in !important;
        margin: 0 auto !important;
        background-image: repeating-linear-gradient(
          to bottom,
          transparent 0,
          transparent calc(11in - 2px),
          rgba(239, 68, 68, 0.45) calc(11in - 2px),
          rgba(239, 68, 68, 0.45) 11in
        );
      }
    }
    @media print {
      @page {
        size: letter;
        margin: ${state.marginIn}in;
      }
      body {
        padding: 0 !important;
        background: none !important;
      }
    }
  `;

    iframe.srcdoc = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <style>${overrideCss}</style>
</head>
<body>${renderedHtml}</body>
</html>`;

    iframe.onload = () => {
        try {
            const doc = iframe.contentDocument;
            if (!doc || !doc.body) return;
            const budget = calculatePageBudget(doc.documentElement.scrollHeight);
            const pill = el<HTMLSpanElement>('page-budget-pill');
            pill.className = `budget-pill ${budget.isSpillover ? 'warning' : 'optimal'}`;
            pill.textContent = budget.label;
        } catch {
            // Ignore cross-origin frame measurements if any
        }
    };
}

async function openVaultFileInEditor(
    category: string,
    filename: string,
    customTargetFilename: string | null = null
): Promise<void> {
    try {
        const content = await readVaultFile(category, filename);
        state.vaultEditor = { category, filename, customTargetFilename };
        el('vault-editor-category').textContent = category;
        el('vault-editor-filename').textContent = filename;

        const promoteBtn = el<HTMLButtonElement>('btn-promote-sample');
        const hint = el<HTMLSpanElement>('vault-editor-hint');
        if (customTargetFilename && filename.endsWith('.sample.md')) {
            promoteBtn.classList.remove('hidden');
            promoteBtn.textContent = `✨ Save as Custom (${customTargetFilename})`;
            hint.textContent =
                'Viewing sample template — click Save as Custom to create your personal git-ignored file.';
        } else {
            promoteBtn.classList.add('hidden');
            hint.textContent = 'Changes take effect immediately on the next agent run.';
        }

        el<HTMLTextAreaElement>('vault-file-textarea').value = content;
        switchStudioMode('vault');
    } catch (err) {
        setStatus('error', `Failed to read ${category}/${filename}: ${String(err)}`);
    }
}

async function refreshVaultOverview(): Promise<void> {
    const overview = await fetchVaultOverview();
    state.overview = overview;
    el('vault-path-label').textContent = overview.root_path;

    // 1. Saved Applications
    const appsList = el<HTMLUListElement>('applications-list');
    appsList.innerHTML = '';
    if (overview.applications.length === 0) {
        const empty = document.createElement('li');
        empty.className = 'nav-item';
        empty.innerHTML = `<span class="nav-item-label" style="color:var(--text-muted)">No saved runs yet</span>`;
        appsList.appendChild(empty);
    } else {
        overview.applications.forEach((appRun) => {
            const li = document.createElement('li');
            li.className = 'nav-item';
            const scoreBadge =
                appRun.match_score != null
                    ? `<span class="badge custom">${appRun.match_score}%</span>`
                    : `<span class="badge inactive">${appRun.status}</span>`;
            li.innerHTML = `<span class="nav-item-label" title="${appRun.slug}">${appRun.title}</span>${scoreBadge}`;
            li.addEventListener('click', async () => {
                try {
                    const ws = await loadApplicationWorkspace(appRun.slug);
                    el<HTMLInputElement>('input-run-title').value = ws.summary.title;
                    el<HTMLTextAreaElement>('input-jd').value = ws.job_description;
                    state.artifacts.draft = ws.draft_markdown;
                    state.artifacts.fact_check = ws.fact_check_markdown;
                    state.artifacts.alignment = ws.alignment_check_markdown;
                    state.artifacts.final = ws.final_resume_markdown || ws.draft_markdown;
                    state.artifacts.cover_letter = ws.cover_letter_markdown;

                    if (ws.archetype_selection) {
                        state.selectedArchetype = ws.archetype_selection.archetype_filename;
                        el<HTMLSelectElement>('select-archetype').value = ws.archetype_selection.archetype_filename;
                        const ratBadge = el('archetype-rationale-badge');
                        ratBadge.textContent = `${ws.archetype_selection.archetype_filename}: ${ws.archetype_selection.rationale}`;
                        ratBadge.classList.remove('hidden');
                    }

                    updateMatchScoreBadge(ws.summary.match_score);
                    switchStudioMode('pipeline');
                    switchArtifactTab('final');
                    setStatus('idle', `Loaded application workspace: ${ws.summary.slug}`);
                } catch (err) {
                    setStatus('error', `Failed to load application: ${String(err)}`);
                }
            });
            appsList.appendChild(li);
        });
    }

    // 2. PII Files
    const piiList = el<HTMLUListElement>('pii-list');
    piiList.innerHTML = '';
    overview.pii_files.forEach((item) => {
        const li = document.createElement('li');
        li.className = 'nav-item';
        const badgeClass = item.is_custom ? 'custom' : 'sample';
        const badgeLabel = item.is_custom ? 'CUSTOM' : 'SAMPLE';
        li.innerHTML = `<span class="nav-item-label">${item.category}</span><span class="badge ${badgeClass}">${badgeLabel}</span>`;
        li.addEventListener('click', () => {
            openVaultFileInEditor('pii', item.active_filename, item.custom_filename);
        });
        piiList.appendChild(li);
    });

    // 3. Archetype Files + Dropdown
    const archList = el<HTMLUListElement>('archetypes-list');
    const archSelect = el<HTMLSelectElement>('select-archetype');
    const previousSelected = archSelect.value;
    archList.innerHTML = '';
    archSelect.innerHTML = `<option value="">🤖 Auto-Select via Archetype Agent</option>`;

    overview.archetype_files.forEach((arch) => {
        const li = document.createElement('li');
        li.className = 'nav-item';
        const badgeClass = arch.is_active_in_selector ? (arch.is_sample ? 'sample' : 'custom') : 'inactive';
        const badgeText = arch.is_active_in_selector ? (arch.is_sample ? 'SAMPLE' : 'ACTIVE') : 'HIDDEN';
        li.innerHTML = `<span class="nav-item-label" title="${arch.filename}">${arch.filename}</span><span class="badge ${badgeClass}">${badgeText}</span>`;
        li.addEventListener('click', () => {
            const targetCustom = arch.is_sample ? arch.filename.replace('.sample.md', '.md') : null;
            openVaultFileInEditor('archetypes', arch.filename, targetCustom);
        });
        archList.appendChild(li);

        const opt = document.createElement('option');
        opt.value = arch.filename;
        opt.textContent = arch.filename;
        archSelect.appendChild(opt);
    });
    archSelect.value = previousSelected;

    // 4. CSS Styles + Dropdown
    const stylesList = el<HTMLUListElement>('styles-list');
    const styleSelect = el<HTMLSelectElement>('select-style-css');
    stylesList.innerHTML = '';
    styleSelect.innerHTML = '';

    overview.style_files.forEach((cssFile) => {
        const li = document.createElement('li');
        li.className = 'nav-item';
        const isSample = cssFile.endsWith('.sample.css');
        li.innerHTML = `<span class="nav-item-label">${cssFile}</span><span class="badge ${isSample ? 'sample' : 'custom'}">${isSample ? 'SAMPLE' : 'CUSTOM'}</span>`;
        li.addEventListener('click', () => {
            const customName = isSample ? cssFile.replace('.sample.css', '.css') : null;
            openVaultFileInEditor('styles', cssFile, customName);
        });
        stylesList.appendChild(li);

        const opt = document.createElement('option');
        opt.value = cssFile;
        opt.textContent = cssFile;
        styleSelect.appendChild(opt);
    });
    if (overview.style_files.includes(state.activeStyleFile)) {
        styleSelect.value = state.activeStyleFile;
    }

    // 5. System Prompts
    const promptsList = el<HTMLUListElement>('prompts-list');
    promptsList.innerHTML = '';
    overview.prompt_files.forEach((promptFile) => {
        const li = document.createElement('li');
        li.className = 'nav-item';
        li.innerHTML = `<span class="nav-item-label">${promptFile}</span>`;
        li.addEventListener('click', () => {
            openVaultFileInEditor('prompts', promptFile, null);
        });
        promptsList.appendChild(li);
    });
}

function updateMatchScoreBadge(score: number | null): void {
    state.matchScore = score;
    const pill = el<HTMLSpanElement>('match-score-pill');
    if (score == null) {
        pill.classList.add('hidden');
    } else {
        pill.classList.remove('hidden');
        pill.textContent = `${score}%`;
    }
}

function setButtonsDisabled(disabled: boolean): void {
    state.isRunning = disabled;
    const ids = [
        'btn-run-full',
        'btn-step-arch',
        'btn-step-draft',
        'btn-step-checks',
        'btn-step-edit',
        'btn-step-pitch'
    ];
    ids.forEach((id) => {
        el<HTMLButtonElement>(id).disabled = disabled;
    });
}

async function initStudio(): Promise<void> {
    // Load default config & vault overview
    state.llmConfig = await fetchDefaultLlmConfig();
    const savedCfg = localStorage.getItem('resumai_llm_config');
    if (savedCfg) {
        try {
            state.llmConfig = { ...state.llmConfig, ...JSON.parse(savedCfg) };
        } catch {
            // Ignore corrupt localStorage
        }
    }
    updateProviderPill();

    await refreshVaultOverview();
    state.activeCssContent = await readStyleCss(state.activeStyleFile);

    // Initialize editor & live preview
    el<HTMLTextAreaElement>('artifact-editor').value = state.artifacts.final;
    renderLivePreview();
    renderMetricPlaceholders();

    // Subscribe to real-time Tauri streaming events
    await subscribeAgentStream((payload) => {
        if (payload.kind === 'thinking') {
            appendThonkLog(payload.chunk, true);
        } else if (payload.kind === 'tool' || payload.kind === 'status') {
            appendThonkLog(`\n[${payload.stage.toUpperCase()}] ${payload.chunk}\n`, false);
        } else if (payload.kind === 'text') {
            // Stream live markdown deltas into the currently active stage buffer if watching it
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
                state.artifacts[targetTab] += payload.chunk;
                if (state.activeArtifactTab === targetTab) {
                    el<HTMLTextAreaElement>('artifact-editor').value = state.artifacts[targetTab];
                }
            }
        }
    });

    bindUiEvents();
}

function bindUiEvents(): void {
    // Mode switching
    el('tab-mode-pipeline').addEventListener('click', () => switchStudioMode('pipeline'));
    el('tab-mode-vault').addEventListener('click', () => switchStudioMode('vault'));

    // Artifact tabs
    document.querySelectorAll<HTMLButtonElement>('.artifact-tab').forEach((btn) => {
        btn.addEventListener('click', () => {
            const tab = btn.dataset.artifact as ArtifactTabKey;
            if (tab) switchArtifactTab(tab);
        });
    });

    // Live Markdown editing updates preview in real time
    el<HTMLTextAreaElement>('artifact-editor').addEventListener('input', (e) => {
        const val = (e.target as HTMLTextAreaElement).value;
        state.artifacts[state.activeArtifactTab] = val;
        renderLivePreview();
        renderMetricPlaceholders();
    });

    // New Clean Application Workspace
    el('btn-new-application').addEventListener('click', () => {
        el<HTMLInputElement>('input-run-title').value = '';
        el<HTMLTextAreaElement>('input-jd').value = '';
        el<HTMLSelectElement>('select-archetype').value = '';
        el('archetype-rationale-badge').classList.add('hidden');
        updateMatchScoreBadge(null);
        state.artifacts = {
            final: '',
            draft: '',
            fact_check: '',
            alignment: '',
            cover_letter: ''
        };
        switchStudioMode('pipeline');
        switchArtifactTab('draft');
        setStatus('idle', 'Started clean application workspace.');
    });

    // Create new custom Archetype or CSS Style
    el('btn-new-archetype').addEventListener('click', async () => {
        const sample = await readVaultFile('archetypes', 'technical_consultant.sample.md').catch(
            () => '# Custom Archetype\n\n## Profile Overview\n'
        );
        state.vaultEditor = {
            category: 'archetypes',
            filename: 'custom_archetype.md',
            customTargetFilename: null
        };
        el('vault-editor-category').textContent = 'archetypes';
        el('vault-editor-filename').textContent = 'custom_archetype.md';
        el('btn-promote-sample').classList.add('hidden');
        el('vault-editor-hint').textContent = 'Creating new custom archetype content/archetypes/custom_archetype.md';
        el<HTMLTextAreaElement>('vault-file-textarea').value = sample;
        switchStudioMode('vault');
    });

    el('btn-new-style').addEventListener('click', async () => {
        const sampleCss = await readStyleCss('resume.sample.css');
        state.vaultEditor = {
            category: 'styles',
            filename: 'resume.custom.css',
            customTargetFilename: null
        };
        el('vault-editor-category').textContent = 'styles';
        el('vault-editor-filename').textContent = 'resume.custom.css';
        el('btn-promote-sample').classList.add('hidden');
        el('vault-editor-hint').textContent = 'Creating custom stylesheet assets/styles/resumes/resume.custom.css';
        el<HTMLTextAreaElement>('vault-file-textarea').value = sampleCss;
        switchStudioMode('vault');
    });

    // Save Vault File
    el('btn-save-vault-file').addEventListener('click', async () => {
        const content = el<HTMLTextAreaElement>('vault-file-textarea').value;
        const { category, filename } = state.vaultEditor;
        try {
            const savedPath = await writeVaultFile(category, filename, content);
            await refreshVaultOverview();
            if (category === 'styles' && filename === state.activeStyleFile) {
                state.activeCssContent = content;
                renderLivePreview();
            }
            setStatus('idle', `Saved ${savedPath}`);
        } catch (err) {
            setStatus('error', `Failed to save file: ${String(err)}`);
        }
    });

    // Promote .sample.md to custom .md
    el('btn-promote-sample').addEventListener('click', async () => {
        const content = el<HTMLTextAreaElement>('vault-file-textarea').value;
        const { category, customTargetFilename } = state.vaultEditor;
        if (!customTargetFilename) return;
        try {
            const savedPath = await writeVaultFile(category, customTargetFilename, content);
            state.vaultEditor.filename = customTargetFilename;
            state.vaultEditor.customTargetFilename = null;
            el('vault-editor-filename').textContent = customTargetFilename;
            el('btn-promote-sample').classList.add('hidden');
            await refreshVaultOverview();
            setStatus('idle', `Created custom override: ${savedPath}`);
        } catch (err) {
            setStatus('error', `Failed to create custom override: ${String(err)}`);
        }
    });

    // Stage 1: Archetype Selector
    el('btn-step-arch').addEventListener('click', async () => {
        const jd = el<HTMLTextAreaElement>('input-jd').value.trim();
        if (!jd) {
            setStatus('error', 'Please paste a Job Description first to run Archetype Selection.');
            return;
        }
        setButtonsDisabled(true);
        clearThonkLog('Stage 1: Resume Archetype Selector');
        setStatus('running', 'Stage 1: Selecting optimal Candidate Archetype via tool calls...');
        try {
            const res = await runSelectArchetype(state.llmConfig, jd);
            state.selectedArchetype = res.archetype_filename;
            state.archetypeRationale = res.rationale;
            el<HTMLSelectElement>('select-archetype').value = res.archetype_filename;
            const badge = el('archetype-rationale-badge');
            badge.textContent = `${res.archetype_filename}: ${res.rationale}`;
            badge.classList.remove('hidden');
            setStatus('idle', `Selected Archetype: ${res.archetype_filename} (${res.model_used})`);
        } catch (err) {
            setStatus('error', `Archetype Selector failed: ${String(err)}`);
        } finally {
            setButtonsDisabled(false);
        }
    });

    // Stage 2: Draft Resume
    el('btn-step-draft').addEventListener('click', async () => {
        const jd = el<HTMLTextAreaElement>('input-jd').value.trim();
        const arch = el<HTMLSelectElement>('select-archetype').value.trim();
        setButtonsDisabled(true);
        state.artifacts.draft = '';
        switchArtifactTab('draft');
        clearThonkLog('Stage 2: Resume Drafter');
        setStatus('running', 'Stage 2: Synthesizing initial Markdown resume draft...');
        try {
            const res = await runDraftResume(state.llmConfig, jd || undefined, arch || undefined);
            state.artifacts.draft = res.markdown;
            el<HTMLTextAreaElement>('artifact-editor').value = res.markdown;
            renderLivePreview();
            renderMetricPlaceholders();
            setStatus('idle', `Draft complete (${res.model_used}). Ready for Stage 3 Parallel Checks.`);
        } catch (err) {
            setStatus('error', `Resume Drafter failed: ${String(err)}`);
        } finally {
            setButtonsDisabled(false);
        }
    });

    // Stage 3: Parallel Fact-Check + Job Alignment Check
    el('btn-step-checks').addEventListener('click', async () => {
        const baseline = state.artifacts.draft.trim() || state.artifacts.final.trim();
        if (!baseline) {
            setStatus('error', 'Generate or paste a Resume Draft first before running checks.');
            return;
        }
        const jd = el<HTMLTextAreaElement>('input-jd').value.trim();
        setButtonsDisabled(true);
        state.artifacts.fact_check = '';
        state.artifacts.alignment = '';
        switchArtifactTab('fact_check');
        clearThonkLog('Stage 3a & 3b: Parallel Fact-Check & Job-Alignment Check (tokio::join!)');
        setStatus('running', 'Stage 3: Running Fact-Checker and Job-Alignment Checker concurrently...');
        try {
            const res = await runParallelChecks(state.llmConfig, baseline, jd || undefined);
            state.artifacts.fact_check = res.fact_check.markdown;
            if (res.alignment_check) {
                state.artifacts.alignment = res.alignment_check.markdown;
                updateMatchScoreBadge(res.alignment_check.match_score);
            }
            el<HTMLTextAreaElement>('artifact-editor').value = state.artifacts.fact_check;
            setStatus('idle', 'Parallel Fact-Check & Alignment Audit complete! Review tabs or run Stage 4.');
        } catch (err) {
            setStatus('error', `Parallel checks failed: ${String(err)}`);
        } finally {
            setButtonsDisabled(false);
        }
    });

    // Stage 4: Resume Editor
    el('btn-step-edit').addEventListener('click', async () => {
        const baseline = state.artifacts.draft.trim() || state.artifacts.final.trim();
        const fc = state.artifacts.fact_check.trim();
        if (!baseline || !fc) {
            setStatus('error', 'Stage 4 Editor requires both a Draft Resume and a Fact-Check Audit report.');
            return;
        }
        const jac = state.artifacts.alignment.trim();
        const notes = el<HTMLInputElement>('input-user-notes').value.trim();
        setButtonsDisabled(true);
        state.artifacts.final = '';
        switchArtifactTab('final');
        clearThonkLog('Stage 4: Resume Editor Synthesis');
        setStatus('running', 'Stage 4: Applying Fact-Check & Job-Alignment revisions...');
        try {
            const res = await runEditResume(state.llmConfig, baseline, fc, jac || undefined, notes || undefined);
            state.artifacts.final = res.markdown;
            el<HTMLTextAreaElement>('artifact-editor').value = res.markdown;
            renderLivePreview();
            renderMetricPlaceholders();
            setStatus('idle', `Final Resume synthesized (${res.model_used})!`);
        } catch (err) {
            setStatus('error', `Resume Editor failed: ${String(err)}`);
        } finally {
            setButtonsDisabled(false);
        }
    });

    // Stage 5: Cover Letter / Consulting Pitch
    el('btn-step-pitch').addEventListener('click', async () => {
        const resumeMd = state.artifacts.final.trim() || state.artifacts.draft.trim();
        if (!resumeMd) {
            setStatus('error', 'Please generate a draft or final resume first.');
            return;
        }
        const jd = el<HTMLTextAreaElement>('input-jd').value.trim();
        const arch = el<HTMLSelectElement>('select-archetype').value.trim();
        const mode = el<HTMLSelectElement>('select-pitch-mode').value;
        setButtonsDisabled(true);
        state.artifacts.cover_letter = '';
        switchArtifactTab('cover_letter');
        clearThonkLog(`Stage 5: ${mode === 'consulting_pitch' ? 'Consulting Proposal' : 'Executive Cover Letter'}`);
        setStatus('running', 'Stage 5: Synthesizing grounded Cover Letter / Consulting Pitch...');
        try {
            const res = await runGenerateCoverLetter(
                state.llmConfig,
                resumeMd,
                jd || undefined,
                arch || undefined,
                mode
            );
            state.artifacts.cover_letter = res.markdown;
            el<HTMLTextAreaElement>('artifact-editor').value = res.markdown;
            renderLivePreview();
            setStatus('idle', `Generated ${mode === 'consulting_pitch' ? 'Consulting Pitch' : 'Cover Letter'}!`);
        } catch (err) {
            setStatus('error', `Cover Letter / Pitch generation failed: ${String(err)}`);
        } finally {
            setButtonsDisabled(false);
        }
    });

    // Full Automated 1-Click Pipeline
    el('btn-run-full').addEventListener('click', async () => {
        const title = el<HTMLInputElement>('input-run-title').value.trim();
        const jd = el<HTMLTextAreaElement>('input-jd').value.trim();
        const archOverride = el<HTMLSelectElement>('select-archetype').value.trim();
        const notes = el<HTMLInputElement>('input-user-notes').value.trim();

        setButtonsDisabled(true);
        state.artifacts = {
            final: '',
            draft: '',
            fact_check: '',
            alignment: '',
            cover_letter: state.artifacts.cover_letter
        };
        switchArtifactTab('draft');
        clearThonkLog('Full End-to-End Pipeline (Archetype -> Draft -> Parallel Checks -> Edit -> Export)');
        setStatus('running', 'Running Full Pipeline (Parallel Checks + Auto-Export)...');

        try {
            const res = await runFullPipeline(
                state.llmConfig,
                title || undefined,
                jd || undefined,
                archOverride || undefined,
                state.activeStyleFile,
                notes || undefined
            );

            if (res.archetype) {
                el<HTMLSelectElement>('select-archetype').value = res.archetype.archetype_filename;
                const badge = el('archetype-rationale-badge');
                badge.textContent = `${res.archetype.archetype_filename}: ${res.archetype.rationale}`;
                badge.classList.remove('hidden');
            }

            state.artifacts.draft = res.draft.markdown;
            state.artifacts.fact_check = res.fact_check.markdown;
            if (res.alignment_check) {
                state.artifacts.alignment = res.alignment_check.markdown;
                updateMatchScoreBadge(res.alignment_check.match_score);
            }
            state.artifacts.final = res.final_resume.markdown;

            switchArtifactTab('final');
            await refreshVaultOverview();

            const banner = el('export-status-banner');
            banner.textContent = `Saved workspace & exported: ${res.exported_docx_path}`;
            banner.classList.remove('hidden');
            setStatus('idle', `Full pipeline complete! Workspace saved to var/applications/${res.slug}`);
        } catch (err) {
            setStatus('error', `Full pipeline failed: ${String(err)}`);
        } finally {
            setButtonsDisabled(false);
        }
    });

    // Preview Typography Sliders & CSS Theme Switcher
    el<HTMLSelectElement>('select-style-css').addEventListener('change', async (e) => {
        state.activeStyleFile = (e.target as HTMLSelectElement).value;
        state.activeCssContent = await readStyleCss(state.activeStyleFile);
        renderLivePreview();
    });

    el<HTMLInputElement>('slider-font-size').addEventListener('input', (e) => {
        state.fontSizePt = parseFloat((e.target as HTMLInputElement).value);
        el('lbl-font-size').textContent = `${state.fontSizePt}pt`;
        renderLivePreview();
    });

    el<HTMLInputElement>('slider-line-height').addEventListener('input', (e) => {
        state.lineHeight = parseFloat((e.target as HTMLInputElement).value);
        el('lbl-line-height').textContent = state.lineHeight.toFixed(2);
        renderLivePreview();
    });

    el<HTMLInputElement>('slider-margin').addEventListener('input', (e) => {
        state.marginIn = parseFloat((e.target as HTMLInputElement).value);
        el('lbl-margin').textContent = `${state.marginIn.toFixed(2)}in`;
        renderLivePreview();
    });

    // Print / PDF via Native WebView
    el('btn-print-pdf').addEventListener('click', () => {
        const iframe = el<HTMLIFrameElement>('preview-iframe');
        if (iframe.contentWindow) {
            iframe.contentWindow.focus();
            iframe.contentWindow.print();
        }
    });

    // Native Rust DOCX & HTML Export
    el('btn-export-bundle').addEventListener('click', async () => {
        const md = state.artifacts[state.activeArtifactTab] || state.artifacts.final || state.artifacts.draft;
        if (!md.trim()) return;
        const title = el<HTMLInputElement>('input-run-title').value.trim();
        const stem = title
            ? title
                  .toLowerCase()
                  .replace(/[^a-z0-9]+/g, '-')
                  .replace(/^-|-$/g, '')
            : 'resume_export';
        try {
            const res = await exportResumeBundle(md, state.activeStyleFile, stem);
            const banner = el('export-status-banner');
            banner.textContent = `Exported DOCX & HTML -> ${res.docx_path}`;
            banner.classList.remove('hidden');
            setStatus('idle', `Exported ${res.docx_path} and ${res.html_path}`);
        } catch (err) {
            setStatus('error', `Export failed: ${String(err)}`);
        }
    });

    // Settings Modal
    el('btn-open-settings').addEventListener('click', () => {
        el<HTMLSelectElement>('cfg-platform').value = state.llmConfig.platform;
        el<HTMLInputElement>('cfg-ollama-url').value = state.llmConfig.ollama_base_url;
        el<HTMLInputElement>('cfg-ollama-model').value = state.llmConfig.ollama_model;
        el<HTMLInputElement>('cfg-gemini-key').value = state.llmConfig.gemini_api_key;
        el<HTMLInputElement>('cfg-gemini-flash').value = state.llmConfig.gemini_flash_model;
        el<HTMLInputElement>('cfg-gemini-lite').value = state.llmConfig.gemini_lite_model;
        el('settings-modal').classList.remove('hidden');
    });

    el('btn-close-settings').addEventListener('click', () => {
        el('settings-modal').classList.add('hidden');
    });

    el('btn-save-settings').addEventListener('click', () => {
        state.llmConfig = {
            platform: el<HTMLSelectElement>('cfg-platform').value as 'ollama' | 'gemini',
            ollama_base_url: el<HTMLInputElement>('cfg-ollama-url').value.trim() || 'http://localhost:11434',
            ollama_model: el<HTMLInputElement>('cfg-ollama-model').value.trim() || 'qwen2.5:14b-instruct-q5_K_M',
            gemini_api_key: el<HTMLInputElement>('cfg-gemini-key').value.trim(),
            gemini_flash_model: el<HTMLInputElement>('cfg-gemini-flash').value.trim() || 'gemini-3.5-flash',
            gemini_lite_model: el<HTMLInputElement>('cfg-gemini-lite').value.trim() || 'gemini-3.5-flash-lite'
        };
        localStorage.setItem('resumai_llm_config', JSON.stringify(state.llmConfig));
        updateProviderPill();
        el('settings-modal').classList.add('hidden');
        setStatus('idle', `Saved AI Provider Settings (${state.llmConfig.platform})`);
    });
}

// Allow automated tests to control DOM lifecycle
if (typeof window !== 'undefined' && !(window as any).__TAURI_TEST_ENV__) {
    window.addEventListener('DOMContentLoaded', () => {
        initStudio().catch((err) => {
            setStatus('error', `Initialization error: ${String(err)}`);
        });
    });
}

export {
    initStudio,
    renderLivePreview,
    bindUiEvents,
    renderMetricPlaceholders,
    openVaultFileInEditor,
    refreshVaultOverview,
    switchStudioMode,
    switchArtifactTab
};

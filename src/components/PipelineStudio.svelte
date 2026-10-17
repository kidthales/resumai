<script lang="ts">
    import { state as studio } from '../lib/state.svelte';
    import MetricsPanel from './MetricsPanel.svelte';
    import ThonkConsole from './ThonkConsole.svelte';

    let titleInputEl: HTMLInputElement | undefined = $state();
    let archetypeSelectEl: HTMLSelectElement | undefined = $state();
    let jdTextareaEl: HTMLTextAreaElement | undefined = $state();
    let pitchModeSelectEl: HTMLSelectElement | undefined = $state();
    let userNotesInputEl: HTMLInputElement | undefined = $state();

    function syncFormDomToStore(): void {
        if (titleInputEl) studio.runTitle = titleInputEl.value;
        if (archetypeSelectEl) studio.selectedArchetype = archetypeSelectEl.value;
        if (jdTextareaEl) studio.jobDescription = jdTextareaEl.value;
        if (pitchModeSelectEl) {
            studio.pitchMode = pitchModeSelectEl.value as 'cover_letter' | 'consulting_pitch';
        }
        if (userNotesInputEl) studio.userNotes = userNotesInputEl.value;
    }
</script>

<div id="view-pipeline" class="workspace-view" class:active={studio.activeMode === 'pipeline'}>
    <!-- Job / Client Engagement Input Panel -->
    <div class="card jd-card">
        <div class="jd-top-row">
            <div class="field-group flex-1">
                <label for="input-run-title">Target Company / Client Engagement Title</label>
                <input
                    bind:this={titleInputEl}
                    id="input-run-title"
                    type="text"
                    placeholder="e.g. Acme Studio — Principal Systems Architect"
                    bind:value={studio.runTitle}
                />
            </div>
            <div class="field-group">
                <label for="select-archetype">Candidate Archetype Lens</label>
                <select bind:this={archetypeSelectEl} id="select-archetype" bind:value={studio.selectedArchetype}>
                    <option value="">🤖 Auto-Select via Archetype Agent</option>
                    {#if studio.overview}
                        {#each studio.overview.archetype_files as arch (arch.filename)}
                            <option value={arch.filename}>{arch.filename}</option>
                        {/each}
                    {/if}
                </select>
            </div>
        </div>

        <div class="field-group">
            <div class="label-row">
                <label for="input-jd">Job Description / Client RFP Brief (Optional for general resume)</label>
                <span
                    id="archetype-rationale-badge"
                    class="rationale-badge"
                    class:hidden={!studio.archetypeRationale}
                >
                    {studio.archetypeRationale}
                </span>
            </div>
            <textarea
                bind:this={jdTextareaEl}
                id="input-jd"
                rows="5"
                placeholder="Paste target Job Description, contract requirements, or client discovery brief here..."
                bind:value={studio.jobDescription}
            ></textarea>
        </div>

        <!-- Pipeline Execution Controls -->
        <div class="pipeline-controls">
            <button
                id="btn-run-full"
                class="btn btn-primary"
                type="button"
                disabled={studio.isRunning}
                onclick={() => {
                    syncFormDomToStore();
                    studio.executeFullPipeline();
                }}
            >
                🚀 Run Full Pipeline (Parallel Checks + Export)
            </button>
            <div class="step-buttons">
                <button
                    id="btn-step-arch"
                    class="btn btn-step"
                    type="button"
                    disabled={studio.isRunning}
                    title="Stage 1: Select best archetype using tool calls"
                    onclick={() => {
                        syncFormDomToStore();
                        studio.executeStage1Archetype();
                    }}
                >
                    1. Archetype
                </button>
                <button
                    id="btn-step-draft"
                    class="btn btn-step"
                    type="button"
                    disabled={studio.isRunning}
                    title="Stage 2: Synthesize initial Markdown draft"
                    onclick={() => {
                        syncFormDomToStore();
                        studio.executeStage2Draft();
                    }}
                >
                    2. Draft
                </button>
                <button
                    id="btn-step-checks"
                    class="btn btn-step"
                    type="button"
                    disabled={studio.isRunning}
                    title="Stage 3a & 3b: Run Fact-Check and Job-Alignment Check concurrently"
                    onclick={() => {
                        syncFormDomToStore();
                        studio.executeStage3ParallelChecks();
                    }}
                >
                    3. Parallel Checks
                </button>
                <button
                    id="btn-step-edit"
                    class="btn btn-step"
                    type="button"
                    disabled={studio.isRunning}
                    title="Stage 4: Apply Fact-Check & Alignment revisions"
                    onclick={() => {
                        syncFormDomToStore();
                        studio.executeStage4Editor();
                    }}
                >
                    4. Edit Final
                </button>
                <button
                    id="btn-step-pitch"
                    class="btn btn-step"
                    type="button"
                    disabled={studio.isRunning}
                    title="Stage 5: Generate verified Cover Letter or Consulting Proposal"
                    onclick={() => {
                        syncFormDomToStore();
                        studio.executeStage5Pitch();
                    }}
                >
                    5. Cover / Pitch
                </button>
                <select
                    bind:this={pitchModeSelectEl}
                    id="select-pitch-mode"
                    class="pitch-mode-select"
                    title="Choose Cover Letter or Consulting Pitch mode"
                    bind:value={studio.pitchMode}
                >
                    <option value="cover_letter">Cover Letter</option>
                    <option value="consulting_pitch">Consulting Pitch</option>
                </select>
            </div>
        </div>
    </div>

    <!-- Live Streaming Progress & .thonk Console -->
    <ThonkConsole />

    <!-- Interactive [METRIC: ...] Placeholder Resolution & Write-Back Panel -->
    <MetricsPanel />

    <!-- Stage Artifact Tabs & Markdown Editor -->
    <div class="card artifacts-card">
        <div class="artifact-tabs-bar">
            <div class="artifact-tabs">
                <button
                    class="artifact-tab"
                    class:active={studio.activeArtifactTab === 'final'}
                    data-artifact="final"
                    type="button"
                    onclick={() => studio.switchArtifactTab('final')}
                >
                    ✨ Final Resume
                </button>
                <button
                    class="artifact-tab"
                    class:active={studio.activeArtifactTab === 'draft'}
                    data-artifact="draft"
                    type="button"
                    onclick={() => studio.switchArtifactTab('draft')}
                >
                    📝 Initial Draft
                </button>
                <button
                    class="artifact-tab"
                    class:active={studio.activeArtifactTab === 'fact_check'}
                    data-artifact="fact_check"
                    type="button"
                    onclick={() => studio.switchArtifactTab('fact_check')}
                >
                    🛡️ Fact-Check Audit
                </button>
                <button
                    class="artifact-tab"
                    class:active={studio.activeArtifactTab === 'alignment'}
                    data-artifact="alignment"
                    type="button"
                    onclick={() => studio.switchArtifactTab('alignment')}
                >
                    🎯 Job Alignment
                    <span id="match-score-pill" class="score-pill" class:hidden={studio.matchScore == null}>
                        {studio.matchScore != null ? `${studio.matchScore}%` : ''}
                    </span>
                </button>
                <button
                    class="artifact-tab"
                    class:active={studio.activeArtifactTab === 'cover_letter'}
                    data-artifact="cover_letter"
                    type="button"
                    onclick={() => studio.switchArtifactTab('cover_letter')}
                >
                    ✉️ Cover Letter / Pitch
                </button>
            </div>
            <div class="artifact-actions">
                <input
                    bind:this={userNotesInputEl}
                    id="input-user-notes"
                    class="notes-input"
                    type="text"
                    placeholder="Optional editorial instruction for Stage 4 Editor..."
                    bind:value={studio.userNotes}
                />
            </div>
        </div>

        <textarea
            id="artifact-editor"
            class="markdown-editor"
            spellcheck="false"
            placeholder="Generated or edited Markdown for the selected stage appears here. Any edits immediately update the live paginated preview on the right..."
            bind:value={studio.artifacts[studio.activeArtifactTab]}
        ></textarea>
    </div>
</div>

<script lang="ts">
    import { flushSync } from 'svelte';
    import { state as studio } from '../lib/state.svelte';
</script>

<header class="topbar">
    <div class="brand">
        <span class="brand-badge">R</span>
        <div>
            <h1>ResumAI Studio <span class="version-pill">v0.2 Tauri</span></h1>
            <p id="vault-path-label" class="subtitle">
                {studio.overview?.root_path || 'Loading local Career Vault...'}
            </p>
        </div>
    </div>

    <div class="topbar-center">
        <button
            id="tab-mode-pipeline"
            class="mode-btn"
            class:active={studio.activeMode === 'pipeline'}
            type="button"
            onclick={() => flushSync(() => studio.switchStudioMode('pipeline'))}
        >
            ⚡ Pipeline &amp; Synthesis Studio
        </button>
        <button
            id="tab-mode-vault"
            class="mode-btn"
            class:active={studio.activeMode === 'vault'}
            type="button"
            onclick={() => flushSync(() => studio.switchStudioMode('vault'))}
        >
            🗂️ Career Vault Markdown Editor
        </button>
    </div>

    <div class="topbar-actions">
        <span id="active-provider-pill" class="provider-pill">{studio.providerPillText}</span>
        <button
            id="btn-open-settings"
            class="btn btn-secondary"
            type="button"
            onclick={() => {
                flushSync(() => {
                    studio.settingsModalOpen = true;
                });
            }}
        >
            ⚙️ AI Provider Settings
        </button>
    </div>
</header>

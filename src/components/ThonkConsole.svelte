<script lang="ts">
    import { state as studio } from '../lib/state.svelte';

    let preEl: HTMLPreElement | undefined = $state();

    $effect(() => {
        void studio.thonkLog;
        if (preEl) {
            preEl.scrollTop = preEl.scrollHeight;
        }
    });
</script>

<details id="thonk-drawer" class="card thonk-card" bind:open={studio.thonkDrawerOpen}>
    <summary class="thonk-summary">
        <div class="thonk-title-group">
            <span id="agent-status-dot" class="status-dot {studio.agentStatus}"></span>
            <strong id="agent-status-text">{studio.agentStatusText}</strong>
        </div>
        <span class="thonk-toggle-hint">Toggle .thonk Reasoning Stream</span>
    </summary>
    <div class="thonk-body">
        <pre bind:this={preEl} id="thonk-stream-log" class="thonk-pre">{studio.thonkLog}</pre>
    </div>
</details>

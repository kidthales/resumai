<script lang="ts">
    import { state as studio } from '../lib/state.svelte';

    let metricValues = $state<Record<string, string>>({});
</script>

<div id="metrics-panel" class="card metrics-card" class:hidden={studio.detectedMetrics.length === 0}>
    <div class="metrics-header">
        <strong>🎯 Unresolved <code>[METRIC: ...]</code> Placeholders Detected</strong>
        <span>
            Fill in your verified numbers to replace inline &amp; optionally save back to
            <code>work_history.md</code>
        </span>
    </div>
    <div id="metrics-list" class="metrics-list">
        {#each studio.detectedMetrics as token (token)}
            <div class="metric-row">
                <span class="metric-token">{token}</span>
                <input
                    type="text"
                    class="flex-1"
                    placeholder="Enter verified metric (e.g. 42% latency reduction)"
                    bind:value={metricValues[token]}
                />
                <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    onclick={() => studio.replaceMetricInActiveTab(token, metricValues[token] || '')}
                >
                    Replace in Resume
                </button>
                <button
                    type="button"
                    class="btn btn-primary btn-sm"
                    onclick={() => studio.saveMetricToWorkHistory(token, metricValues[token] || '')}
                >
                    💾 Save Fact to work_history.md
                </button>
            </div>
        {/each}
    </div>
</div>

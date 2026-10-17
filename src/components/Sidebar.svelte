<script lang="ts">
    import { state as studio } from '../lib/state.svelte';
</script>

<aside class="sidebar" style="width: {studio.sidebarWidthPx}px; flex-shrink: 0;">
    <div class="sidebar-section">
        <div class="section-header">
            <h2>Saved Applications</h2>
            <button
                id="btn-new-application"
                class="btn-mini"
                type="button"
                title="Start a clean pipeline workspace"
                onclick={() => studio.startNewApplication()}
            >
                + New
            </button>
        </div>
        <ul id="applications-list" class="nav-list">
            {#if !studio.overview || studio.overview.applications.length === 0}
                <li class="nav-item">
                    <span class="nav-item-label" style="color:var(--text-muted)">No saved runs yet</span>
                </li>
            {:else}
                {#each studio.overview.applications as appRun (appRun.slug)}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                    <li class="nav-item" onclick={() => studio.loadSavedApplication(appRun.slug)}>
                        <span class="nav-item-label" title={appRun.slug}>{appRun.title}</span>
                        {#if appRun.match_score != null}
                            <span class="badge custom">{appRun.match_score}%</span>
                        {:else}
                            <span class="badge inactive">{appRun.status}</span>
                        {/if}
                    </li>
                {/each}
            {/if}
        </ul>
    </div>

    <div class="sidebar-section">
        <div class="section-header">
            <h2>Candidate PII Vault</h2>
            <span class="hint-pill">100% Local</span>
        </div>
        <ul id="pii-list" class="nav-list">
            {#if studio.overview}
                {#each studio.overview.pii_files as item (item.category)}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                    <li
                        class="nav-item"
                        onclick={() =>
                            studio.openVaultFileInEditor('pii', item.active_filename, item.custom_filename)}
                    >
                        <span class="nav-item-label">{item.category}</span>
                        <span class="badge {item.is_custom ? 'custom' : 'sample'}">
                            {item.is_custom ? 'CUSTOM' : 'SAMPLE'}
                        </span>
                    </li>
                {/each}
            {/if}
        </ul>
    </div>

    <div class="sidebar-section">
        <div class="section-header">
            <h2>Candidate Archetypes</h2>
            <button
                id="btn-new-archetype"
                class="btn-mini"
                type="button"
                title="Create a new custom archetype .md file"
                onclick={() => studio.startNewArchetype()}
            >
                + New
            </button>
        </div>
        <ul id="archetypes-list" class="nav-list">
            {#if studio.overview}
                {#each studio.overview.archetype_files as arch (arch.filename)}
                    {@const badgeClass = arch.is_active_in_selector
                        ? arch.is_sample
                            ? 'sample'
                            : 'custom'
                        : 'inactive'}
                    {@const badgeText = arch.is_active_in_selector
                        ? arch.is_sample
                            ? 'SAMPLE'
                            : 'ACTIVE'
                        : 'HIDDEN'}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                    <li
                        class="nav-item"
                        onclick={() => {
                            const targetCustom = arch.is_sample
                                ? arch.filename.replace('.sample.md', '.md')
                                : null;
                            studio.openVaultFileInEditor('archetypes', arch.filename, targetCustom);
                        }}
                    >
                        <span class="nav-item-label" title={arch.filename}>{arch.filename}</span>
                        <span class="badge {badgeClass}">{badgeText}</span>
                    </li>
                {/each}
            {/if}
        </ul>
    </div>

    <div class="sidebar-section">
        <div class="section-header">
            <h2>CSS Resume Styles</h2>
            <button
                id="btn-new-style"
                class="btn-mini"
                type="button"
                title="Create a custom CSS theme"
                onclick={() => studio.startNewStyle()}
            >
                + New
            </button>
        </div>
        <ul id="styles-list" class="nav-list">
            {#if studio.overview}
                {#each studio.overview.style_files as cssFile (cssFile)}
                    {@const isSample = cssFile.endsWith('.sample.css')}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                    <li
                        class="nav-item"
                        onclick={() => {
                            const customName = isSample ? cssFile.replace('.sample.css', '.css') : null;
                            studio.openVaultFileInEditor('styles', cssFile, customName);
                        }}
                    >
                        <span class="nav-item-label">{cssFile}</span>
                        <span class="badge {isSample ? 'sample' : 'custom'}">
                            {isSample ? 'SAMPLE' : 'CUSTOM'}
                        </span>
                    </li>
                {/each}
            {/if}
        </ul>
    </div>

    <div class="sidebar-section">
        <div class="section-header">
            <h2>Agent System Prompts</h2>
        </div>
        <ul id="prompts-list" class="nav-list">
            {#if studio.overview}
                {#each studio.overview.prompt_files as promptFile (promptFile)}
                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                    <li
                        class="nav-item"
                        onclick={() => studio.openVaultFileInEditor('prompts', promptFile, null)}
                    >
                        <span class="nav-item-label">{promptFile}</span>
                    </li>
                {/each}
            {/if}
        </ul>
    </div>
</aside>

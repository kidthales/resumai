<script lang="ts">
    import { state as studio } from '../lib/state.svelte';

    let vaultTextareaEl: HTMLTextAreaElement | undefined = $state();

    function syncTextareaToStore(): void {
        if (vaultTextareaEl) {
            studio.vaultFileContent = vaultTextareaEl.value;
        }
    }

    let showPromoteButton = $derived(
        Boolean(studio.vaultEditor.customTargetFilename && studio.vaultEditor.filename.endsWith('.sample.md'))
    );
</script>

<div id="view-vault" class="workspace-view" class:active={studio.activeMode === 'vault'}>
    <div class="card vault-editor-card">
        <div class="vault-editor-header">
            <div>
                <span id="vault-editor-category" class="category-badge">{studio.vaultEditor.category}</span>
                <strong id="vault-editor-filename">{studio.vaultEditor.filename}</strong>
                <span id="vault-editor-hint" class="vault-hint">{studio.vaultEditorHint}</span>
            </div>
            <div class="vault-editor-actions">
                <button
                    id="btn-promote-sample"
                    class="btn btn-secondary"
                    class:hidden={!showPromoteButton}
                    type="button"
                    onclick={() => {
                        syncTextareaToStore();
                        studio.promoteSampleVaultFile();
                    }}
                >
                    {studio.vaultEditor.customTargetFilename
                        ? `✨ Save as Custom (${studio.vaultEditor.customTargetFilename})`
                        : '✨ Save as Custom Override (.md)'}
                </button>
                <button
                    id="btn-save-vault-file"
                    class="btn btn-primary"
                    type="button"
                    onclick={() => {
                        syncTextareaToStore();
                        studio.saveCurrentVaultFile();
                    }}
                >
                    💾 Save Vault File
                </button>
            </div>
        </div>
        <textarea
            bind:this={vaultTextareaEl}
            id="vault-file-textarea"
            class="markdown-editor full-height"
            spellcheck="false"
            bind:value={studio.vaultFileContent}
        ></textarea>
    </div>
</div>

<script lang="ts">
    import { flushSync } from 'svelte';
    import { state as studio } from '../lib/state.svelte';

    let dialogEl: HTMLDialogElement | undefined = $state();
    let platformSelectEl: HTMLSelectElement | undefined = $state();
    let ollamaUrlInputEl: HTMLInputElement | undefined = $state();
    let ollamaModelInputEl: HTMLInputElement | undefined = $state();
    let geminiKeyInputEl: HTMLInputElement | undefined = $state();
    let geminiFlashInputEl: HTMLInputElement | undefined = $state();
    let geminiLiteInputEl: HTMLInputElement | undefined = $state();

    let draftPlatform = $state<'ollama' | 'gemini'>('ollama');
    let draftOllamaUrl = $state('http://localhost:11434');
    let draftOllamaModel = $state('qwen2.5:14b-instruct-q5_K_M');
    let draftGeminiKey = $state('');
    let draftGeminiFlash = $state('gemini-3.5-flash');
    let draftGeminiLite = $state('gemini-3.5-flash-lite');

    $effect(() => {
        if (studio.settingsModalOpen) {
            draftPlatform = studio.llmConfig.platform;
            draftOllamaUrl = studio.llmConfig.ollama_base_url;
            draftOllamaModel = studio.llmConfig.ollama_model;
            draftGeminiKey = studio.llmConfig.gemini_api_key;
            draftGeminiFlash = studio.llmConfig.gemini_flash_model;
            draftGeminiLite = studio.llmConfig.gemini_lite_model;
            if (dialogEl && typeof dialogEl.showModal === 'function' && !dialogEl.open) {
                try {
                    dialogEl.showModal();
                } catch {
                    dialogEl.setAttribute('open', '');
                }
            }
        } else if (dialogEl && dialogEl.open) {
            if (typeof dialogEl.close === 'function') {
                dialogEl.close();
            } else {
                dialogEl.removeAttribute('open');
            }
        }
    });

    function handleSave(): void {
        flushSync(() => {
            studio.saveSettings({
                platform: (platformSelectEl?.value as 'ollama' | 'gemini') || draftPlatform,
                ollama_base_url: ollamaUrlInputEl ? ollamaUrlInputEl.value : draftOllamaUrl,
                ollama_model: ollamaModelInputEl ? ollamaModelInputEl.value : draftOllamaModel,
                gemini_api_key: geminiKeyInputEl ? geminiKeyInputEl.value : draftGeminiKey,
                gemini_flash_model: geminiFlashInputEl ? geminiFlashInputEl.value : draftGeminiFlash,
                gemini_lite_model: geminiLiteInputEl ? geminiLiteInputEl.value : draftGeminiLite
            });
        });
    }

    function handleClose(): void {
        flushSync(() => {
            studio.settingsModalOpen = false;
        });
    }
</script>

<dialog
    bind:this={dialogEl}
    id="settings-modal"
    class="modal-backdrop"
    class:hidden={!studio.settingsModalOpen}
    oncancel={(e) => {
        e.preventDefault();
        handleClose();
    }}
>
    <div class="modal-card">
        <div class="modal-header">
            <h2>⚙️ AI Inference Provider Settings</h2>
            <button id="btn-close-settings" class="btn-icon" type="button" onclick={handleClose}>✕</button>
        </div>
        <div class="modal-body">
            <div class="field-group">
                <label for="cfg-platform">Active Agent Platform</label>
                <select bind:this={platformSelectEl} id="cfg-platform" bind:value={draftPlatform}>
                    <option value="ollama">Local Ollama (100% On-Device Inference)</option>
                    <option value="gemini">Google Gemini API (Cloud Flash / Flash-Lite)</option>
                </select>
            </div>

            <div class="field-group">
                <label for="cfg-ollama-url">Ollama Base URL</label>
                <input
                    bind:this={ollamaUrlInputEl}
                    id="cfg-ollama-url"
                    type="text"
                    placeholder="http://localhost:11434"
                    bind:value={draftOllamaUrl}
                />
            </div>

            <div class="field-group">
                <label for="cfg-ollama-model">Ollama Model Name</label>
                <input
                    bind:this={ollamaModelInputEl}
                    id="cfg-ollama-model"
                    type="text"
                    placeholder="qwen2.5:14b-instruct-q5_K_M"
                    bind:value={draftOllamaModel}
                />
            </div>

            <hr class="modal-divider" />

            <div class="field-group">
                <label for="cfg-gemini-key">Google Gemini API Key (Stored locally on your machine)</label>
                <input
                    bind:this={geminiKeyInputEl}
                    id="cfg-gemini-key"
                    type="password"
                    placeholder="AIzaSy..."
                    bind:value={draftGeminiKey}
                />
            </div>

            <div class="jd-top-row">
                <div class="field-group flex-1">
                    <label for="cfg-gemini-flash">Gemini Flash Model (Fact-Check &amp; Editor)</label>
                    <input
                        bind:this={geminiFlashInputEl}
                        id="cfg-gemini-flash"
                        type="text"
                        bind:value={draftGeminiFlash}
                    />
                </div>
                <div class="field-group flex-1">
                    <label for="cfg-gemini-lite">Gemini Lite Model (Archetype, Draft &amp; Align)</label>
                    <input
                        bind:this={geminiLiteInputEl}
                        id="cfg-gemini-lite"
                        type="text"
                        bind:value={draftGeminiLite}
                    />
                </div>
            </div>
        </div>
        <div class="modal-footer">
            <button id="btn-save-settings" class="btn btn-primary" type="button" onclick={handleSave}>
                Save Provider Configuration
            </button>
        </div>
    </div>
</dialog>

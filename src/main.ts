import { flushSync, mount, unmount } from 'svelte';
import App from './App.svelte';
import { state } from './lib/state.svelte';
import type { ArtifactTabKey } from './types';

let mountedApp: Record<string, any> | null = null;

export async function initStudio(): Promise<void> {
    if (mountedApp) {
        try {
            unmount(mountedApp);
        } catch {
            // Ignore unmount errors if container was already cleared
        }
        mountedApp = null;
    }

    const target = document.getElementById('app') || document.body;
    target.innerHTML = '';

    flushSync(() => {
        state.reset();
        mountedApp = mount(App, { target });
    });

    await state.init();
    flushSync();
}

export function switchStudioMode(mode: 'pipeline' | 'vault'): void {
    flushSync(() => {
        state.switchStudioMode(mode);
    });
}

export function switchArtifactTab(tab: ArtifactTabKey): void {
    flushSync(() => {
        state.switchArtifactTab(tab);
    });
}

export async function openVaultFileInEditor(
    category: string,
    filename: string,
    customTargetFilename: string | null = null
): Promise<void> {
    await state.openVaultFileInEditor(category, filename, customTargetFilename);
    flushSync();
}

export async function refreshVaultOverview(): Promise<void> {
    await state.refreshVaultOverview();
    flushSync();
}

export function renderLivePreview(): void {
    flushSync();
}

export function renderMetricPlaceholders(): void {
    flushSync();
}

export function bindUiEvents(): void {
    // Declaratively handled by Svelte 5 component bindings
}

// Allow automated tests to control DOM lifecycle
if (typeof window !== 'undefined' && !(window as any).__TAURI_TEST_ENV__) {
    window.addEventListener('DOMContentLoaded', () => {
        initStudio().catch((err) => {
            state.setStatus('error', `Initialization error: ${String(err)}`);
        });
    });
}

export { state };

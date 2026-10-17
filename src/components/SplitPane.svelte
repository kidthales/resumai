<script lang="ts">
    import { state as studio } from '../lib/state.svelte';
    import PaperPreview from './PaperPreview.svelte';
    import PipelineStudio from './PipelineStudio.svelte';
    import Sidebar from './Sidebar.svelte';
    import VaultEditor from './VaultEditor.svelte';

    let draggingGutter = $state<'left' | 'right' | null>(null);
    let containerEl: HTMLElement | undefined = $state();

    function persistWidths(): void {
        try {
            localStorage.setItem(
                'resumai_pane_widths',
                JSON.stringify({
                    sidebar: studio.sidebarWidthPx,
                    preview: studio.previewWidthPx
                })
            );
        } catch {
            // Ignore localStorage quota/disabled errors
        }
    }

    function startDrag(gutter: 'left' | 'right', e: PointerEvent): void {
        draggingGutter = gutter;
        (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    }

    function onDragMove(e: PointerEvent): void {
        if (!draggingGutter || !containerEl) return;
        const rect = containerEl.getBoundingClientRect();
        if (draggingGutter === 'left') {
            const nextLeft = Math.round(Math.max(180, Math.min(450, e.clientX - rect.left)));
            studio.sidebarWidthPx = nextLeft;
        } else if (draggingGutter === 'right') {
            const nextRight = Math.round(Math.max(340, Math.min(780, rect.right - e.clientX)));
            studio.previewWidthPx = nextRight;
        }
    }

    function stopDrag(): void {
        if (draggingGutter) {
            draggingGutter = null;
            persistWidths();
        }
    }
</script>

<main bind:this={containerEl} class="split-pane-container">
    <Sidebar />

    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
        id="gutter-left"
        class="split-gutter"
        class:dragging={draggingGutter === 'left'}
        role="separator"
        tabindex="0"
        aria-orientation="vertical"
        aria-label="Resize Vault Sidebar"
        onpointerdown={(e) => startDrag('left', e)}
        onpointermove={onDragMove}
        onpointerup={stopDrag}
        onkeydown={(e) => {
            if (e.key === 'ArrowLeft') {
                studio.sidebarWidthPx = Math.max(180, studio.sidebarWidthPx - 16);
                persistWidths();
            } else if (e.key === 'ArrowRight') {
                studio.sidebarWidthPx = Math.min(450, studio.sidebarWidthPx + 16);
                persistWidths();
            }
        }}
    ></div>

    <section class="center-workspace" style="flex: 1;">
        <PipelineStudio />
        <VaultEditor />
    </section>

    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
        id="gutter-right"
        class="split-gutter"
        class:dragging={draggingGutter === 'right'}
        role="separator"
        tabindex="0"
        aria-orientation="vertical"
        aria-label="Resize Paper Preview"
        onpointerdown={(e) => startDrag('right', e)}
        onpointermove={onDragMove}
        onpointerup={stopDrag}
        onkeydown={(e) => {
            if (e.key === 'ArrowLeft') {
                studio.previewWidthPx = Math.min(780, studio.previewWidthPx + 16);
                persistWidths();
            } else if (e.key === 'ArrowRight') {
                studio.previewWidthPx = Math.max(340, studio.previewWidthPx - 16);
                persistWidths();
            }
        }}
    ></div>

    <PaperPreview />
</main>

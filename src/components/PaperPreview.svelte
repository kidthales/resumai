<script lang="ts">
    import { marked } from 'marked';
    import { state as studio } from '../lib/state.svelte';

    let iframeEl: HTMLIFrameElement | undefined = $state();

    let renderedHtml = $derived(marked.parse(studio.previewMarkdown, { async: false }) as string);

    let overrideCss = $derived(`
    ${studio.activeCssContent}
    html, body {
      font-size: ${studio.fontSizePt}pt !important;
      line-height: ${studio.lineHeight} !important;
    }
    @media screen {
      body {
        padding: ${studio.marginIn}in !important;
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
        margin: ${studio.marginIn}in;
      }
      body {
        padding: 0 !important;
        background: none !important;
      }
    }
  `);

    let srcdoc = $derived(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <style>${overrideCss}</style>
</head>
<body>${renderedHtml}</body>
</html>`);

    function handleIframeLoad(): void {
        try {
            const doc = iframeEl?.contentDocument;
            if (!doc || !doc.body) return;
            studio.iframeScrollHeight = doc.documentElement.scrollHeight;
        } catch {
            // Ignore cross-origin frame measurements if any
        }
    }

    function printPdf(): void {
        if (iframeEl?.contentWindow) {
            iframeEl.contentWindow.focus();
            iframeEl.contentWindow.print();
        }
    }
</script>

<aside class="preview-pane" style="width: {studio.previewWidthPx}px; flex-shrink: 0;">
    <div class="preview-toolbar">
        <div class="budget-group">
            <span class="budget-label">Page Budget:</span>
            <span
                id="page-budget-pill"
                class="budget-pill {studio.pageBudget.isSpillover ? 'warning' : 'optimal'}"
            >
                {studio.pageBudget.label}
            </span>
            <select
                id="select-target-pages"
                class="target-pages-select"
                title="Target Page Budget"
                value={String(studio.targetPages)}
                onchange={(e) => {
                    const val = parseInt((e.currentTarget as HTMLSelectElement).value, 10);
                    studio.targetPages = val === 1 ? 1 : 2;
                }}
            >
                <option value="1">Target: 1 Page</option>
                <option value="2">Target: 2 Pages</option>
            </select>
            <button
                id="btn-auto-fit-page"
                class="btn {studio.pageBudget.needsRefinement ? 'btn-primary' : 'btn-secondary'} btn-sm"
                type="button"
                disabled={studio.isRunning}
                title="Automatically tighten or expand phrasing with AI to fit target page budget exactly"
                onclick={() => studio.autoFitPageBudget()}
            >
                📐 Auto-Fit Budget
            </button>
        </div>

        <div class="export-buttons">
            <button
                id="btn-print-pdf"
                class="btn btn-primary btn-sm"
                type="button"
                title="Print or Save as PDF using native WebView print engine"
                onclick={printPdf}
            >
                🖨️ Print / PDF
            </button>
            <button
                id="btn-export-bundle"
                class="btn btn-secondary btn-sm"
                type="button"
                title="Export standalone .docx and .html via Rust ExportEngine"
                onclick={() => studio.exportBundle()}
            >
                📦 Export DOCX &amp; HTML
            </button>
        </div>
    </div>

    <!-- Typography & CSS Theme Controls -->
    <div class="preview-controls-bar">
        <label class="control-item">
            <span>Theme</span>
            <select
                id="select-style-css"
                value={studio.activeStyleFile}
                onchange={(e) => studio.changeStyleFile((e.currentTarget as HTMLSelectElement).value)}
            >
                {#if studio.overview}
                    {#each studio.overview.style_files as cssFile (cssFile)}
                        <option value={cssFile}>{cssFile}</option>
                    {/each}
                {/if}
            </select>
        </label>
        <label class="control-item">
            <span>Font <strong id="lbl-font-size">{studio.fontSizePt}pt</strong></span>
            <input
                id="slider-font-size"
                type="range"
                min="8.5"
                max="11.5"
                step="0.25"
                bind:value={studio.fontSizePt}
            />
            <input
                type="number"
                class="stepper-number"
                min="8.5"
                max="11.5"
                step="0.25"
                aria-label="Font size numeric stepper"
                bind:value={studio.fontSizePt}
            />
        </label>
        <label class="control-item">
            <span>Line <strong id="lbl-line-height">{studio.lineHeight.toFixed(2)}</strong></span>
            <input
                id="slider-line-height"
                type="range"
                min="1.15"
                max="1.55"
                step="0.05"
                bind:value={studio.lineHeight}
            />
            <input
                type="number"
                class="stepper-number"
                min="1.15"
                max="1.55"
                step="0.05"
                aria-label="Line height numeric stepper"
                bind:value={studio.lineHeight}
            />
        </label>
        <label class="control-item">
            <span>Margin <strong id="lbl-margin">{studio.marginIn.toFixed(2)}in</strong></span>
            <input
                id="slider-margin"
                type="range"
                min="0.3"
                max="0.65"
                step="0.05"
                bind:value={studio.marginIn}
            />
            <input
                type="number"
                class="stepper-number"
                min="0.3"
                max="0.65"
                step="0.05"
                aria-label="Margin numeric stepper"
                bind:value={studio.marginIn}
            />
        </label>
    </div>

    <div id="export-status-banner" class="export-banner" class:hidden={!studio.exportBannerText}>
        {studio.exportBannerText}
    </div>

    <!-- Live Letter Paper Preview Frame -->
    <div class="paper-stage">
        <iframe
            bind:this={iframeEl}
            id="preview-iframe"
            title="Live Paginated Resume Preview"
            {srcdoc}
            onload={handleIframeLoad}
        ></iframe>
    </div>
</aside>

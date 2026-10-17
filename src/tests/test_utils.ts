import fs from 'node:fs';
import path from 'node:path';

/**
 * Loads index.html into happy-dom document body while stripping
 * script and stylesheet tags to prevent external resource warnings.
 */
export function loadStudioHtml(): void {
    const raw = fs.readFileSync(path.resolve(__dirname, '../../index.html'), 'utf-8');
    const cleaned = raw
        .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
        .replace(/<link\b[^>]*>/gi, '');
    document.body.innerHTML = cleaned;
}

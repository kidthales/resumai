import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';
import { defineConfig } from 'vitest/config';

const host = process.env.TAURI_DEV_HOST;

export default defineConfig({
    plugins: [svelte(), svelteTesting()],
    clearScreen: false,
    server: {
        port: 1420,
        strictPort: true,
        host: host || false,
        hmr: host
            ? {
                  protocol: 'ws',
                  host,
                  port: 1421
              }
            : undefined,
        watch: {
            ignored: ['**/src-tauri/**', '**/var/**']
        }
    },
    test: {
        environment: 'happy-dom',
        globals: true,
        setupFiles: ['./src/tests/setup.ts'],
        include: ['src/tests/**/*.test.ts']
    }
});


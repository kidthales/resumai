import { defineConfig } from 'vitest/config';

const host = process.env.TAURI_DEV_HOST;

export default defineConfig({
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

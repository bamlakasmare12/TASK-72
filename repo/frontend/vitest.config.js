import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

const testsDir = fileURLToPath(new URL('../tests/unit/frontend', import.meta.url));

export default defineConfig({
  plugins: [react()],
  server: {
    fs: {
      allow: [fileURLToPath(new URL('..', import.meta.url))],
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./test-setup.js'],
    include: [testsDir + '/**/*.{test,spec}.{js,jsx,ts,tsx}'],
  },
});

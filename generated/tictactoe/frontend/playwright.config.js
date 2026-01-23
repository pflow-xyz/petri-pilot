import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.js',
  timeout: 60000,
  use: {
    baseURL: 'http://localhost:8080',
    headless: false,
    viewport: { width: 1280, height: 720 },
  },
})

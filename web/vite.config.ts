import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

// The House frame's font (the family picker, Manage) is only discovered once
// React renders, so it lands after first paint and the swap reflows the page.
// Preload its Latin subset from the built assets so it arrives with the JS.
const PRELOAD_FONTS = [/^onest-latin-wght-normal-[\w-]+\.woff2$/]

function preloadFonts(): Plugin {
  return {
    name: 'openchore:preload-fonts',
    apply: 'build',
    transformIndexHtml: {
      order: 'post',
      handler(_html, ctx) {
        const files = Object.keys(ctx.bundle ?? {})
        return PRELOAD_FONTS.flatMap((re) => files.filter((f) => re.test(f.split('/').pop() ?? '')))
          .map((href) => ({
            tag: 'link',
            attrs: { rel: 'preload', as: 'font', type: 'font/woff2', href: `/${href}`, crossorigin: '' },
            injectTo: 'head' as const,
          }))
      },
    },
  }
}

// https://vitejs.dev/config/
// VITE_API_TARGET lets parallel e2e runs point at their own API server.
const apiTarget = process.env.VITE_API_TARGET || 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), preloadFonts()],
  server: {
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
      },
      '/uploads': {
        target: apiTarget,
        changeOrigin: true,
      },
      '/tts': {
        target: apiTarget,
        changeOrigin: true,
      }
    }
  },
  test: {
    globals: true,
    // Default to node; component tests opt into jsdom via a
    // `// @vitest-environment jsdom` docblock at the top of the file.
    environment: 'node',
  },
})

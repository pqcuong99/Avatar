import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

// `game-server` only resolves inside Docker Compose. For `npm run dev` on the host,
// set GAME_SERVER_URL (e.g. http://localhost:8081) in the shell or in client/.env.local.
const DEFAULT_GAME_SERVER_URL = 'http://game-server:8081';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'GAME_SERVER_');
  const target = env.GAME_SERVER_URL || process.env.GAME_SERVER_URL || DEFAULT_GAME_SERVER_URL;

  return {
    plugins: [react()],
    build: {
      // Phaser is large and changes rarely: keep it in its own cacheable chunk.
      chunkSizeWarningLimit: 1600,
      rollupOptions: { output: { manualChunks: { phaser: ['phaser'] } } },
    },
    resolve: {
      // Maps, tileset and protocol fixtures are shared with the Go servers.
      alias: { '@shared': fileURLToPath(new URL('../shared', import.meta.url)) },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      strictPort: true,
      fs: { allow: ['..'] },
      // Same-origin API and WebSocket: the session cookie stays first-party.
      proxy: {
        '/api': { target, changeOrigin: true, xfwd: true },
        '/admin/api': { target, changeOrigin: true, xfwd: true },
        '/ws': { target: target.replace(/^http/, 'ws'), ws: true, changeOrigin: true },
      },
    },
  };
});

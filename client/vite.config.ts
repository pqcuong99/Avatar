import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

// `game-server` only resolves inside Docker Compose. For `npm run dev` on the host,
// set GAME_SERVER_URL (e.g. http://localhost:8081) in the shell or in client/.env.local.
const DEFAULT_GAME_SERVER_URL = 'http://game-server:8081';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'GAME_SERVER_');

  return {
    plugins: [react()],
    server: {
      host: '0.0.0.0',
      port: 5173,
      strictPort: true,
      proxy: {
        '/game-api': {
          target: env.GAME_SERVER_URL || DEFAULT_GAME_SERVER_URL,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/game-api/, ''),
        },
      },
    },
  };
});

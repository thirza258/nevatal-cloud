import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Exact node_modules path segments, so e.g. `react-dom` never matches a `react-*` package.
const REACT_VENDOR_RE = /[\\/]node_modules[\\/](react|react-dom|scheduler)[\\/]/;
const LUCIDE_VENDOR_RE = /[\\/]node_modules[\\/]lucide-react[\\/]/;

export default defineConfig({
  plugins: [react()],
  build: {
    target: 'es2022',
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (REACT_VENDOR_RE.test(id)) return 'vendor-react';
          if (LUCIDE_VENDOR_RE.test(id)) return 'vendor-lucide';
          return undefined;
        },
      },
    },
  },
  server: {
    port: 3080,
    host: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: true,
      },
    },
  },
});

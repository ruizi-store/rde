import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";

export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  root: resolve(__dirname, "src"),
  base: "/app/android/",
  resolve: {
    alias: {
      $lib: resolve(__dirname, "src"),
      $shared: resolve(__dirname, "../../../frontend/src/shared"),
      "svelte-i18n": resolve(__dirname, "node_modules/svelte-i18n"),
    },
  },
  build: {
    outDir: resolve(__dirname, "dist"),
    emptyOutDir: true,
    target: "esnext",
    rollupOptions: {
      input: resolve(__dirname, "src/index.html"),
    },
  },
  server: {
    port: 5203,
    proxy: {
      "/api": {
        target: "http://localhost:3080",
        changeOrigin: true,
        ws: true,
      },
      "/ws": {
        target: "ws://localhost:3080",
        ws: true,
      },
    },
  },
});

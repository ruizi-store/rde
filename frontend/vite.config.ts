import tailwindcss from "@tailwindcss/vite";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  build: {
    // 图标 JSON 数据文件较大（~7MB），作为懒加载 chunk 是预期行为
    chunkSizeWarningLimit: 5000,
  },
  optimizeDeps: {
    // noVNC 使用 top-level await，不要预构建
    exclude: ["@novnc/novnc"],
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:3080",
        changeOrigin: true,
        ws: true, // 支持 WebSocket 代理
      },
      "/ws": { target: "ws://localhost:3080", ws: true },
    },
  },
});

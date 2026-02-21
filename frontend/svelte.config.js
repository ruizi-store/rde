import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  compilerOptions: {
    // 忽略一些误报警告
    warningFilter: (warning) => {
      // 忽略所有 a11y 警告（桌面应用不需要无障碍检查）
      if (warning.code?.startsWith("a11y_") || warning.code?.startsWith("a11y-")) return false;
      // 忽略导出函数的 non_reactive_update 警告（这是误报）
      if (warning.code === "non_reactive_update") return false;
      return true;
    },
  },
  kit: {
    adapter: adapter({
      pages: "build",
      assets: "build",
      fallback: "index.html",
      precompress: false,
      strict: true,
    }),
    alias: {
      $apps: "src/apps",
      $desktop: "src/desktop",
      $shared: "src/shared",
    },
  },
};

export default config;

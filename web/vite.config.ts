import vue from "@vitejs/plugin-vue";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [vue()],
  build: {
    emptyOutDir: false,
    outDir: "web/static",
    rollupOptions: {
      input: fileURLToPath(new URL("./src/main.ts", import.meta.url)),
      output: {
        entryFileNames: "app.js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: (assetInfo) => {
          if (assetInfo.name?.endsWith(".css")) return "style.css";
          return "assets/[name][extname]";
        },
      },
    },
  },
});

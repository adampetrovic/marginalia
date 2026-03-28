import { defineConfig } from "vite";

export default defineConfig({
  build: {
    target: "esnext",
    minify: "esbuild",
    lib: {
      entry: "src/index.ts",
      formats: ["iife"],
      name: "MarginaliaPlugin",
      fileName: () => "index.js",
    },
  },
});

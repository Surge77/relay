import { defineConfig } from "vitest/config";

// Unit tests cover pure logic (store, protocol) and need neither a DOM nor the
// app's Tailwind/PostCSS pipeline. An inline empty PostCSS config stops Vite
// from loading postcss.config.mjs (whose Tailwind v4 plugin isn't test-loadable).
export default defineConfig({
  css: { postcss: { plugins: [] } },
  test: {
    environment: "node",
    include: ["lib/**/*.test.ts"],
  },
});

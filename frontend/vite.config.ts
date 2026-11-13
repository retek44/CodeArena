import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { configDefaults } from "vitest/config";

// In development and preview the API is reached through a proxy, so the browser sees
// one origin (like CloudFront in AWS) and no CORS configuration is needed.
const apiProxy = {
  "/api": { target: process.env.CODEARENA_API_URL ?? "http://localhost:8080", changeOrigin: true },
};

export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true, proxy: apiProxy },
  preview: { port: 4173, strictPort: true, proxy: apiProxy },
  // Monaco chunks are large but load lazily, only on problem pages.
  build: { sourcemap: true, chunkSizeWarningLimit: 3000 },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    // Pact consumer tests run with their own config (npm run test:pact).
    exclude: [...configDefaults.exclude, "src/**/*.pact.test.ts"],
    restoreMocks: true,
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/test/**", "src/api/pact/**", "src/api/schema.d.ts", "src/main.tsx", "src/vite-env.d.ts"],
      // lcov for SonarCloud, text summary for the job log.
      reporter: ["text-summary", "lcov"],
    },
  },
});

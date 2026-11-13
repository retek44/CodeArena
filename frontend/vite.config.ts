/// <reference types="vitest/config" />
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

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
    restoreMocks: true,
  },
});

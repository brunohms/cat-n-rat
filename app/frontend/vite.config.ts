import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// In development the backend is reached through this proxy, so the browser only talks to the Vite origin
const backendUrl = process.env.BACKEND_URL ?? `http://localhost:${process.env.BACKEND_PORT ?? "8001"}`;

export default defineConfig({
  plugins: [react()],
  server: {
    port: Number(process.env.FRONTEND_PORT ?? 5173),
    proxy: {
      "/api": {
        target: backendUrl,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
  },
});

import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");

  return {
    plugins: [react()],
    server: {
      host: "0.0.0.0",
      port: Number(env.WEB_PORT ?? "51740"),
      allowedHosts: ["nova-quant.wjxconline.com"],
    },
    preview: {
      allowedHosts: ["nova-quant.wjxconline.com"],
    },
    test: {
      globals: true,
      environment: "jsdom",
      setupFiles: "./src/test/setup.ts",
      include: ["src/**/*.test.tsx"],
      exclude: ["tests/e2e/**"],
    },
  };
});

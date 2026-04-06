import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const proxyTarget =
    env.VITE_PROXY_TARGET || env.VITE_API_ORIGIN || "http://localhost:8080/";

  return {
    plugins: [react()],
    publicDir: "favicon",
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: proxyTarget
        ? {
            "/master": {
              target: proxyTarget,
              changeOrigin: true,
              secure: true,
            },
          }
        : undefined,
    },
  };
});

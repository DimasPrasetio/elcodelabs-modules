import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": path.resolve(__dirname, "src") } },
  build: {
    rollupOptions: {
      output: {
        // Libraries change less often than the app: in their own chunks they stay cached
        // across deploys.
        manualChunks: {
          react: ["react", "react-dom", "react-router-dom"],
          ui: ["radix-ui", "lucide-react"],
        },
      },
    },
  },
  server: {
    port: 5173,
    // Same origin in dev, same as the single-container production model.
    proxy: {
      "/api": {
        target: process.env.VITE_DEV_API_TARGET ?? "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});

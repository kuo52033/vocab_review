import { defineConfig, loadEnv, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

function manifestEnvPlugin(apiHostPermission: string): Plugin {
  return {
    name: "manifest-env",
    async writeBundle(options) {
      const outputDir = options.dir ?? "dist";
      const manifestPath = resolve(outputDir, "manifest.json");
      const source = await readFile(manifestPath, "utf8");
      await writeFile(manifestPath, source.replace("__API_HOST_PERMISSION__", apiHostPermission));
    }
  };
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiURL = env.VITE_API_URL ?? "http://localhost:8080";
  const apiHostPermission = `${apiURL.replace(/\/$/, "")}/*`;

  return {
    plugins: [react(), manifestEnvPlugin(apiHostPermission)],
    build: {
      rollupOptions: {
        input: {
          popup: resolve(__dirname, "popup.html"),
          background: resolve(__dirname, "src/background.ts")
        },
        output: {
          entryFileNames: "assets/[name].js"
        }
      }
    }
  };
});

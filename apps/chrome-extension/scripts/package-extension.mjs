import { createWriteStream } from "node:fs";
import { mkdir, readdir, rm, stat } from "node:fs/promises";
import { basename, dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const extensionRoot = resolve(scriptDir, "..");
const distDir = join(extensionRoot, "dist");
const releaseDir = join(extensionRoot, "release");
const zipPath = join(releaseDir, "vocab-review-capture.zip");

async function collectFiles(dir) {
  const entries = await readdir(dir);
  const files = [];
  for (const entry of entries) {
    const path = join(dir, entry);
    const info = await stat(path);
    if (info.isDirectory()) {
      files.push(...await collectFiles(path));
    } else {
      files.push(path);
    }
  }
  return files;
}

async function zipExtension() {
  await mkdir(releaseDir, { recursive: true });
  await rm(zipPath, { force: true });
  await new Promise((resolvePromise, rejectPromise) => {
    const child = spawn("zip", ["-r", "-X", "-q", zipPath, "."], {
      cwd: distDir,
      stdio: "inherit"
    });
    child.on("error", rejectPromise);
    child.on("exit", (code) => code === 0 ? resolvePromise() : rejectPromise(new Error(`zip exited with ${code}`)));
  });
}

async function writeManifest() {
  const files = await collectFiles(distDir);
  const manifestPath = join(releaseDir, "vocab-review-capture-files.txt");
  const manifest = files
    .map((file) => relative(distDir, file))
    .sort()
    .join("\n");
  const stream = createWriteStream(manifestPath);
  stream.end(`${manifest}\n`);
}

await zipExtension();
await writeManifest();
console.log(`Packaged ${relative(extensionRoot, zipPath)}`);

import { rm, readdir, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const appRoot = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const dist = path.resolve(appRoot, "../internal/web/dist");

await mkdir(dist, { recursive: true });
for (const entry of await readdir(dist)) {
  if (entry === ".gitkeep") {
    continue;
  }
  await rm(path.join(dist, entry), { force: true, recursive: true });
}

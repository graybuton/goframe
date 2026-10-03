import { readFile } from "node:fs/promises";
import { createServer } from "node:net";
import { join } from "node:path";

function requireArguments(command, args, minimum, maximum = minimum) {
  if (args.length < minimum || args.length > maximum) {
    const expected = minimum === maximum ? `${minimum}` : `${minimum} or more`;
    const suffix = minimum === 1 && maximum === 1 ? "" : "s";
    throw new Error(`${command} requires ${expected} argument${suffix}`);
  }
}

function errorMessage(error) {
  return error instanceof Error ? error.message : String(error);
}

async function validateJSON(files) {
  requireArguments("validate-json", files, 1, Number.POSITIVE_INFINITY);
  for (const file of files) {
    const contents = await readFile(file, "utf8");
    try {
      JSON.parse(contents);
    } catch (error) {
      throw new Error(`${file}: ${errorMessage(error)}`);
    }
  }
}

async function pickFreePort(args) {
  requireArguments("pick-free-port", args, 0);
  const port = await new Promise((resolve, reject) => {
    const server = createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        server.close();
        reject(new Error("could not determine the selected TCP port"));
        return;
      }
      server.close((error) => {
        if (error) {
          reject(error);
          return;
        }
        resolve(address.port);
      });
    });
  });
  console.log(port);
}

async function manifestWASMPath(args) {
  requireArguments("manifest-wasm-path", args, 1);
  const manifestPath = join(
    args[0],
    ".goframe",
    "package",
    "standalone",
    "asset-manifest.json",
  );
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  if (!manifest.entrypoints || !manifest.entrypoints.wasm) {
    throw new Error(`missing entrypoints.wasm in ${manifestPath}`);
  }
  console.log(manifest.entrypoints.wasm);
}

const [command, ...args] = process.argv.slice(2);

try {
  switch (command) {
    case "validate-json":
      await validateJSON(args);
      break;
    case "pick-free-port":
      await pickFreePort(args);
      break;
    case "manifest-wasm-path":
      await manifestWASMPath(args);
      break;
    default:
      throw new Error(`unknown command: ${command || "<missing>"}`);
  }
} catch (error) {
  console.error(`ci-node-tools: ${errorMessage(error)}`);
  process.exitCode = 1;
}

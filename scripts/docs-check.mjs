#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, normalize, resolve } from "node:path";

const root = resolve(dirname(new URL(import.meta.url).pathname), "..");

function fail(message) {
    console.error(`docs check: ${message}`);
    process.exitCode = 1;
}

function walk(dir, predicate, out = []) {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) {
            if (entry.name === ".git" || entry.name === ".goframe" || entry.name === "node_modules") {
                continue;
            }
            walk(path, predicate, out);
            continue;
        }
        if (entry.isFile() && predicate(path)) {
            out.push(path);
        }
    }
    return out;
}

function stripFencedCode(markdown) {
    return markdown.replace(/^```[\s\S]*?^```/gm, "");
}

function checkMarkdownLinks() {
    const markdownFiles = [
        join(root, "README.md"),
        ...walk(join(root, "docs"), (path) => path.endsWith(".md")),
        ...walk(join(root, "examples"), (path) => path.endsWith("README.md")),
    ];

    const linkPattern = /!?\[[^\]\n]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;

    for (const file of markdownFiles) {
        const markdown = stripFencedCode(readFileSync(file, "utf8"));
        for (const match of markdown.matchAll(linkPattern)) {
            let target = match[1].trim();
            if (
                target === "" ||
                target.startsWith("#") ||
                target.startsWith("http://") ||
                target.startsWith("https://") ||
                target.startsWith("mailto:")
            ) {
                continue;
            }

            if (target.startsWith("<") && target.endsWith(">")) {
                target = target.slice(1, -1);
            }

            const [pathPart] = target.split("#", 1);
            if (pathPart === "") {
                continue;
            }

            let decoded;
            try {
                decoded = decodeURI(pathPart);
            } catch {
                fail(`${relative(file)} contains an invalid link target: ${target}`);
                continue;
            }

            const absolute = decoded.startsWith("/")
                ? join(root, decoded)
                : normalize(join(dirname(file), decoded));

            if (!existsSync(absolute)) {
                fail(`${relative(file)} links to missing ${target}`);
            }
        }
    }
}

function checkExamples() {
    const examplesDir = join(root, "examples");
    const readme = readFileSync(join(root, "README.md"), "utf8");
    const exampleDirs = readdirSync(examplesDir)
        .map((name) => join(examplesDir, name))
        .filter((path) => statSync(path).isDirectory())
        .filter((path) => existsSync(join(path, "goframe.json")))
        .sort();

    for (const dir of exampleDirs) {
        const name = relative(dir);
        if (!existsSync(join(dir, "README.md"))) {
            fail(`${name} has goframe.json but no README.md`);
        }
        if (!readme.includes(name)) {
            fail(`README.md does not mention ${name}`);
        }
    }
}

function relative(path) {
    return path.startsWith(root + "/") ? path.slice(root.length + 1) : path;
}

checkMarkdownLinks();
checkExamples();

if (process.exitCode) {
    process.exit(process.exitCode);
}

console.log("docs check: ok");

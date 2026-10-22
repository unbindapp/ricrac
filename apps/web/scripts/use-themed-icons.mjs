import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import ts from "typescript";

const root = new URL("..", import.meta.url).pathname;
const srcDir = join(root, "src");
const iconsDir = join(srcDir, "components/icons");
const iconsModule = "@/components/icons";
const symbolMap = JSON.parse(readFileSync(join(iconsDir, "symbol-map.json"), "utf8"));
const types = new Set(["LucideIcon", "LucideProps"]);

function walk(dir) {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (full === iconsDir) return [];
    if (statSync(full).isDirectory()) return walk(full);
    if (!/\.tsx?$/.test(entry) || entry.includes(".gen.")) return [];
    return [full];
  });
}

function specifiersOf(declaration) {
  const clause = declaration.importClause;
  if (!clause?.namedBindings || !ts.isNamedImports(clause.namedBindings)) return null;
  return clause.namedBindings.elements.map((element) => ({
    imported: element.propertyName?.text ?? element.name.text,
    local: element.name.text,
    isType: clause.isTypeOnly || element.isTypeOnly,
  }));
}

function importText(specifiers, source) {
  const unique = [
    ...new Map(specifiers.map((specifier) => [specifier.local, specifier])).values(),
  ].sort((a, b) => a.imported.toLowerCase().localeCompare(b.imported.toLowerCase()));
  const allTypes = unique.every((specifier) => specifier.isType);
  const names = unique.map(({ imported, local, isType }) => {
    const alias = local === imported ? "" : ` as ${local}`;
    return `${isType && !allTypes ? "type " : ""}${imported}${alias}`;
  });
  return `import ${allTypes ? "type " : ""}{ ${names.join(", ")} } from "${source}";`;
}

/** Files without @/ imports stay runnable by node's tests, so they get a relative path. */
function iconsSourceFor(file, imports) {
  const usesAlias = imports.some((declaration) => declaration.moduleSpecifier.text.startsWith("@/"));
  if (usesAlias || file.endsWith(".tsx")) return iconsModule;
  const path = relative(dirname(file), join(iconsDir, "index.ts"));
  return path.startsWith(".") ? path : `./${path}`;
}

function lineEnd(text, position) {
  return text[position] === "\n" ? position + 1 : position;
}

function insertion(imports, source, merged) {
  const sourceOf = (declaration) => declaration.moduleSpecifier.text;
  const isLocal = (declaration) => /^(@\/|\.)/.test(sourceOf(declaration));
  const local = imports.filter(isLocal);
  const after = local.find((declaration) => sourceOf(declaration).localeCompare(source) > 0);
  if (after) return { start: after.getStart(), end: after.getStart(), text: `${merged}\n` };
  if (local.length > 0) {
    const last = local.at(-1).getEnd();
    return { start: last, end: last, text: `\n${merged}` };
  }
  const last = imports.at(-1).getEnd();
  return { start: last, end: last, text: `\n\n${merged}` };
}

function migrate(file, problems) {
  const text = readFileSync(file, "utf8");
  if (!text.includes("lucide-react")) return null;
  const kind = file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const sourceFile = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, kind);
  const where = (node) =>
    `${relative(root, file)}:${sourceFile.getLineAndCharacterOfPosition(node.getStart()).line + 1}`;

  const imports = sourceFile.statements.filter(ts.isImportDeclaration);
  const lucideImports = imports.filter((d) => d.moduleSpecifier.text === "lucide-react");
  if (lucideImports.length === 0) return null;
  const rest = imports.filter((declaration) => !lucideImports.includes(declaration));
  const source = iconsSourceFor(file, rest);

  const specifiers = [];
  for (const declaration of lucideImports) {
    const named = specifiersOf(declaration);
    if (!named) {
      problems.push(`${where(declaration)}: only named imports from lucide-react are supported`);
      continue;
    }
    for (const specifier of named) {
      if (!types.has(specifier.imported) && !symbolMap[specifier.imported]) {
        problems.push(
          `${where(declaration)}: ${specifier.imported} is not in src/components/icons/symbol-map.json (add it, then bun run symbols)`,
        );
      }
    }
    specifiers.push(...named);
  }

  const removals = lucideImports.map((declaration) => ({
    start: declaration.getStart(),
    end: lineEnd(text, declaration.getEnd()),
    text: "",
  }));
  const existing = rest.find((declaration) => declaration.moduleSpecifier.text === source);
  const merged = importText([...(existing ? specifiersOf(existing) : []), ...specifiers], source);
  const edits = existing
    ? [...removals, { start: existing.getStart(), end: existing.getEnd(), text: merged }]
    : rest.length > 0
      ? [...removals, insertion(rest, source, merged)]
      : [{ ...removals[0], text: `${merged}\n` }, ...removals.slice(1)];

  return edits
    .sort((a, b) => b.start - a.start)
    .reduce((acc, edit) => acc.slice(0, edit.start) + edit.text + acc.slice(edit.end), text);
}

const problems = [];
const changes = walk(srcDir)
  .map((file) => [file, migrate(file, problems)])
  .filter(([, next]) => next !== null);

if (problems.length > 0) {
  for (const problem of problems) console.error(problem);
  console.error("Nothing written.");
  process.exit(1);
}

for (const [file, next] of changes) writeFileSync(file, next);
const changed = changes.map(([file]) => file);
if (changed.length > 0) {
  execFileSync(
    join(root, "node_modules/.bin/prettier"),
    ["--write", "--log-level", "warn", ...changed],
    { cwd: root, stdio: "inherit" },
  );
}
console.log(`Moved ${changed.length} files from lucide-react to ${iconsModule}.`);

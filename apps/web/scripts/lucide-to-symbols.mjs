import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import ts from "typescript";

const root = new URL("..", import.meta.url).pathname;
const srcDir = join(root, "src");
const symbolsModule = "@/components/icons/symbols.gen";
const generatedFile = join(srcDir, "components/icons/symbols.gen.ts");
const mapFile = join(srcDir, "components/icons/symbol-map.json");
const typeRenames = { LucideIcon: "TIcon", LucideProps: "TIconProps" };
const droppedProps = new Set(["strokeWidth", "absoluteStrokeWidth"]);

const symbolMap = JSON.parse(readFileSync(mapFile, "utf8"));
const generatedExports = new Set(
  [...readFileSync(generatedFile, "utf8").matchAll(/^export (?:const|type) (\w+)/gm)].map(
    (match) => match[1],
  ),
);

const isSymbolsSource = (source) => /(^|\/)symbols\.gen(\.ts)?$/.test(source);

function walk(dir) {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) return walk(full);
    if (!/\.tsx?$/.test(entry) || entry.includes(".gen.")) return [];
    return [full];
  });
}

function importsOf(sourceFile, matches) {
  return sourceFile.statements.filter(
    (statement) => ts.isImportDeclaration(statement) && matches(statement.moduleSpecifier.text),
  );
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

function specifierText({ imported, local, isType }, inline) {
  const alias = local === imported ? "" : ` as ${local}`;
  return `${isType && inline ? "type " : ""}${imported}${alias}`;
}

function importText(specifiers, source) {
  const unique = [
    ...new Map(specifiers.map((specifier) => [specifier.local, specifier])).values(),
  ].sort((a, b) => a.imported.toLowerCase().localeCompare(b.imported.toLowerCase()));
  const allTypes = unique.every((specifier) => specifier.isType);
  const names = unique.map((specifier) => specifierText(specifier, !allTypes)).join(", ");
  return `import ${allTypes ? "type " : ""}{ ${names} } from "${source}";`;
}

function lineEnd(text, position) {
  return text[position] === "\n" ? position + 1 : position;
}

function importEdits(sourceFile, text, lucideImports, symbolImports) {
  const lucideSpecifiers = lucideImports.flatMap(specifiersOf).map((specifier) => {
    const renamed = typeRenames[specifier.imported] ?? specifier.imported;
    const local = specifier.local === specifier.imported ? renamed : specifier.local;
    return { ...specifier, imported: renamed, local };
  });
  const [existing] = symbolImports;
  const source = existing?.moduleSpecifier.text ?? symbolsModule;
  const merged = importText(
    [...(existing ? specifiersOf(existing) : []), ...lucideSpecifiers],
    source,
  );
  const removals = lucideImports.map((declaration) => ({
    start: declaration.getStart(),
    end: lineEnd(text, declaration.getEnd()),
    text: "",
  }));

  if (existing) {
    return [...removals, { start: existing.getStart(), end: existing.getEnd(), text: merged }];
  }

  const imports = sourceFile.statements.filter(
    (statement) => ts.isImportDeclaration(statement) && !lucideImports.includes(statement),
  );
  const sourceOf = (declaration) => declaration.moduleSpecifier.text;
  const aliased = imports.filter((declaration) => sourceOf(declaration).startsWith("@/"));
  const after = aliased.find((declaration) => sourceOf(declaration).localeCompare(source) > 0);
  if (after)
    return [...removals, { start: after.getStart(), end: after.getStart(), text: `${merged}\n` }];
  if (aliased.length > 0) {
    const last = aliased.at(-1).getEnd();
    return [...removals, { start: last, end: last, text: `\n${merged}` }];
  }

  const firstRelative = imports.find((declaration) => sourceOf(declaration).startsWith("."));
  if (firstRelative) {
    const start = firstRelative.getStart();
    return [...removals, { start, end: start, text: `${merged}\n\n` }];
  }
  if (imports.length > 0) {
    const last = imports.at(-1).getEnd();
    return [...removals, { start: last, end: last, text: `\n\n${merged}` }];
  }
  const [first, ...rest] = removals;
  return [...rest, { ...first, text: `${merged}\n` }];
}

function attributeValue(attribute) {
  return attribute.initializer ? attribute.initializer.getText() : "{true}";
}

function iconElementEdits(element, warn) {
  const attributes = element.attributes.properties.filter(ts.isJsxAttribute);
  const hasFilled = attributes.some((attribute) => attribute.name.getText() === "filled");
  return attributes.flatMap((attribute) => {
    const name = attribute.name.getText();
    if (droppedProps.has(name)) {
      return [{ start: attribute.getFullStart(), end: attribute.getEnd(), text: "" }];
    }
    if (name === "size") {
      const value = attributeValue(attribute);
      return [
        {
          start: attribute.getStart(),
          end: attribute.getEnd(),
          text: `width=${value} height=${value}`,
        },
      ];
    }
    if (name !== "className") return [];
    const initializer = attribute.initializer;
    if (!initializer || !ts.isStringLiteral(initializer)) {
      if (/\bfill-/.test(attribute.getText())) warn(attribute, "computed className with fill-*");
      return [];
    }
    const classes = initializer.text.split(/\s+/).filter(Boolean);
    const otherFills = classes.filter((cls) => cls !== "fill-current" && /(^|:)fill-/.test(cls));
    if (otherFills.length > 0) warn(attribute, `fill classes ${otherFills.join(" ")} (use filled)`);
    if (!classes.includes("fill-current")) return [];
    const rest = classes.filter((cls) => cls !== "fill-current").join(" ");
    const filled = hasFilled ? "" : "filled ";
    return [
      {
        start: attribute.getStart(),
        end: attribute.getEnd(),
        text: `${filled}className="${rest}"`,
      },
    ];
  });
}

function migrate(file, problems, warnings) {
  const text = readFileSync(file, "utf8");
  if (!text.includes("lucide-react") && !/symbols\.gen/.test(text)) return null;
  const sourceFile = ts.createSourceFile(
    file,
    text,
    ts.ScriptTarget.Latest,
    true,
    file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const where = (node) => {
    const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart());
    return `${relative(root, file)}:${line + 1}`;
  };
  const warn = (node, message) => warnings.push(`${where(node)}: ${message}`);

  const lucideImports = importsOf(sourceFile, (source) => source === "lucide-react");
  const symbolImports = importsOf(sourceFile, isSymbolsSource);
  for (const declaration of lucideImports) {
    const specifiers = specifiersOf(declaration);
    if (!specifiers) {
      problems.push(`${where(declaration)}: only named imports from lucide-react are supported`);
      continue;
    }
    for (const { imported } of specifiers) {
      if (typeRenames[imported] || generatedExports.has(imported)) continue;
      const fix = symbolMap[imported] ? "run bun run symbols" : "add it to symbol-map.json";
      problems.push(`${where(declaration)}: ${imported} has no symbol (${fix})`);
    }
  }

  const imported = [...lucideImports, ...symbolImports].flatMap((d) => specifiersOf(d) ?? []);
  const iconNames = new Set(imported.filter((s) => !s.isType).map((s) => s.local));
  const renamedTypes = new Set(
    lucideImports
      .flatMap((d) => specifiersOf(d) ?? [])
      .filter((s) => typeRenames[s.imported] && s.local === s.imported)
      .map((s) => s.local),
  );

  const edits = [];
  const visit = (node) => {
    if (ts.isImportDeclaration(node)) return;
    if (ts.isIdentifier(node) && renamedTypes.has(node.text)) {
      edits.push({ start: node.getStart(), end: node.getEnd(), text: typeRenames[node.text] });
    }
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText();
      if (iconNames.has(tag)) edits.push(...iconElementEdits(node, warn));
      else if (/Icon$/.test(tag) && /\bstrokeWidth=/.test(node.attributes.getText())) {
        warn(node, `strokeWidth on ${tag}, check whether it renders a symbol`);
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  if (lucideImports.length > 0)
    edits.push(...importEdits(sourceFile, text, lucideImports, symbolImports));
  if (edits.length === 0) return null;

  const next = edits
    .sort((a, b) => b.start - a.start)
    .reduce((acc, edit) => acc.slice(0, edit.start) + edit.text + acc.slice(edit.end), text);
  return next === text ? null : next;
}

const problems = [];
const warnings = [];
const changes = walk(srcDir)
  .map((file) => [file, migrate(file, problems, warnings)])
  .filter(([, next]) => next !== null);

for (const warning of warnings) console.warn(`warning ${warning}`);
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
    {
      cwd: root,
      stdio: "inherit",
    },
  );
}
console.log(`Migrated ${changed.length} files.`);

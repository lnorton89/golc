import postcss from "postcss";

const CONTROLLED_CLASS = /(button|field|dialog|tab|toolbar|chip|badge|empty|loading|error|focus)/i;
const SHARED_VISUAL_PROPERTIES = new Set(["color", "background", "background-color", "padding", "margin", "border", "border-radius", "outline", "font-size", "font-family", "font-weight", "line-height", "transition", "box-shadow", "z-index"]);
const RAW_VISUAL_PROPERTIES = /^(color|background(?:-color)?|border(?:-(?:color|radius))?|padding(?:-.+)?|margin(?:-.+)?|gap|font-(?:size|family|weight)|line-height|transition|animation|box-shadow|z-index|outline)$/;
const SAFE_LITERALS = new Set(["0", "0px", "none", "auto", "inherit", "initial", "unset", "transparent", "currentColor", "normal"]);

function location(node) {
  return { line: node.source?.start?.line ?? 1, column: node.source?.start?.column ?? 1 };
}

function diagnostic(rule, path, node, message, value = "") {
  return { rule, path, ...location(node), message, value };
}

function hasRawLiteral(value) {
  const normalized = value.trim();
  return !SAFE_LITERALS.has(normalized) && !/^var\(--ds-[a-z0-9-]+\)$/.test(normalized) && !/^calc\(var\(--ds-[a-z0-9-]+\)[+*/ -]*\)$/.test(normalized);
}

function customProperties(value) {
  return [...value.matchAll(/var\(\s*(--[A-Za-z0-9-]+)\s*(?:,\s*[^)]+)?\)/g)].map((match) => ({ name: match[1], fallback: match[0].includes(",") }));
}

export function checkCSS({ path, source, declaredTokens = new Set(), isDesignSystemFile = false, isSafetyFile = false }) {
  const diagnostics = [];
  let root;
  try {
    root = postcss.parse(source, { from: path });
  } catch (error) {
    const line = error.line ?? 1;
    const column = error.column ?? 1;
    return [{ rule: "DS000", path, line, column, message: "CSS syntax error", value: error.reason ?? "" }];
  }
  root.walkRules((rule) => {
    if (!isDesignSystemFile && /\[data-theme(?:-name)?(?:=|\])/i.test(rule.selector)) diagnostics.push(diagnostic("DS004", path, rule, "theme selector", rule.selector));
    if (!isDesignSystemFile && /(^|[\s>+~,])(button|input|select|textarea)(?=\b|[.#[:])/i.test(rule.selector)) diagnostics.push(diagnostic("DS005", path, rule, "styled native control", rule.selector));
    const classNames = [...rule.selector.matchAll(/\.([A-Za-z0-9_-]+)/g)].map((match) => match[1]);
    if (!isDesignSystemFile && classNames.some((name) => CONTROLLED_CLASS.test(name))) {
      const shared = rule.nodes?.some((node) => node.type === "decl" && SHARED_VISUAL_PROPERTIES.has(node.prop)) ?? false;
      if (shared) diagnostics.push(diagnostic("DS006", path, rule, "shared visual class", rule.selector));
    }
    if (isSafetyFile && /(blackout|revoke|safety)/i.test(rule.selector) && /(?:display\s*:\s*none|visibility\s*:\s*hidden|(?:width|height)\s*:\s*0(?:px)?)/i.test(rule.toString())) diagnostics.push(diagnostic("DS010", path, rule, "safety visibility", rule.selector));
  });
  root.walkDecls((decl) => {
    if (!isDesignSystemFile && decl.prop.startsWith("--")) diagnostics.push(diagnostic("DS002", path, decl, "custom property declaration", decl.prop));
    for (const reference of customProperties(decl.value)) {
      if (!declaredTokens.has(reference.name) && !reference.name.startsWith("--ds-")) diagnostics.push(diagnostic("DS003", path, decl, "unknown custom property", reference.name));
      if (!isDesignSystemFile && reference.fallback) diagnostics.push(diagnostic("DS003", path, decl, "custom property fallback", reference.name));
    }
    if (!isDesignSystemFile && RAW_VISUAL_PROPERTIES.test(decl.prop) && hasRawLiteral(decl.value)) diagnostics.push(diagnostic("DS001", path, decl, "raw visual literal", `${decl.prop}: ${decl.value}`));
    if (!isDesignSystemFile && decl.prop === "outline" && /^(none|0(?:px)?)$/i.test(decl.value.trim())) diagnostics.push(diagnostic("DS010", path, decl, "focus", decl.value));
    if (!isDesignSystemFile && decl.prop === "transition" && !/var\(--ds-motion-/.test(decl.value) && decl.value.trim() !== "none") diagnostics.push(diagnostic("DS010", path, decl, "raw motion", decl.value));
  });
  return diagnostics;
}

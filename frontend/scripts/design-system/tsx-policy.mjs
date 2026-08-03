import ts from "@typescript/typescript6";

function position(source, node) {
  const at = source.getLineAndCharacterOfPosition(node.getStart(source));
  return { line: at.line + 1, column: at.character + 1 };
}

export function checkTSX({ path, source, isPrimitiveFile = false, isThemeFile = false }) {
  const file = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const diagnostics = [];
  for (const parse of file.parseDiagnostics) {
    const at = source.getLineAndCharacterOfPosition(parse.start ?? 0);
    diagnostics.push({ rule: "DS000", path, line: at.line + 1, column: at.character + 1, message: "TSX syntax error", value: ts.flattenDiagnosticMessageText(parse.messageText, " ") });
  }
  const add = (rule, node, message, value = "") => diagnostics.push({ rule, path, ...position(file, node), message, value });
  const visit = (node) => {
    if ((ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) && ts.isIdentifier(node.tagName) && /^(button|input|select|textarea)$/.test(node.tagName.text) && !isPrimitiveFile) {
      const styled = node.attributes.properties.some((property) => ts.isJsxAttribute(property) && (property.name.text === "className" || property.name.text === "style"));
      if (styled) add("DS005", node, "styled native control", node.tagName.text);
    }
    if (!isThemeFile && ts.isStringLiteral(node) && /^(dark|light|default|gruvbox|tokyo|dracula|nord|catppuccin|solarized|one-dark|rose-pine|everforest|rainbow|acid)$/.test(node.text)) {
      const parent = node.parent;
      if (ts.isBinaryExpression(parent) || ts.isConditionalExpression(parent) || ts.isSwitchStatement(parent.parent)) add("DS004", node, "theme branch", node.text);
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return diagnostics;
}

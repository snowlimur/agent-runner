import type { JSONValue, PipelineConditionScope } from "./types.js";

type TokenType =
  | "identifier"
  | "number"
  | "string"
  | "boolean"
  | "null"
  | "operator"
  | "paren_open"
  | "paren_close"
  | "dot"
  | "comma";

interface Token {
  type: TokenType;
  value: string;
  position: number;
}

type ASTNode =
  | { type: "literal"; value: JSONValue }
  | { type: "path"; segments: string[] }
  | { type: "binary"; operator: string; left: ASTNode; right: ASTNode };

interface ParserState {
  tokens: Token[];
  index: number;
}

const OPERATORS = ["==", "!=", ">=", "<=", "&&", "||", ">", "<"];

function tokenize(input: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;

  while (i < input.length) {
    const ch = input[i];
    if (!ch) {
      break;
    }

    if (/\s/.test(ch)) {
      i += 1;
      continue;
    }

    if (ch === "(") {
      tokens.push({ type: "paren_open", value: ch, position: i });
      i += 1;
      continue;
    }
    if (ch === ")") {
      tokens.push({ type: "paren_close", value: ch, position: i });
      i += 1;
      continue;
    }
    if (ch === ".") {
      tokens.push({ type: "dot", value: ch, position: i });
      i += 1;
      continue;
    }
    if (ch === ",") {
      tokens.push({ type: "comma", value: ch, position: i });
      i += 1;
      continue;
    }

    const pair = input.slice(i, i + 2);
    if (OPERATORS.includes(pair)) {
      tokens.push({ type: "operator", value: pair, position: i });
      i += 2;
      continue;
    }
    if (OPERATORS.includes(ch)) {
      tokens.push({ type: "operator", value: ch, position: i });
      i += 1;
      continue;
    }

    if (ch === '"' || ch === "'") {
      const quote = ch;
      let j = i + 1;
      let escaped = false;
      while (j < input.length) {
        const current = input[j];
        if (!current) {
          break;
        }
        if (escaped) {
          escaped = false;
          j += 1;
          continue;
        }
        if (current === "\\") {
          escaped = true;
          j += 1;
          continue;
        }
        if (current === quote) {
          break;
        }
        j += 1;
      }
      if (j >= input.length || input[j] !== quote) {
        throw new Error(`Unterminated string literal at position ${i}`);
      }

      const raw = input.slice(i, j + 1);
      const normalizedRaw = quote === "'" ? `"${raw.slice(1, -1).replace(/\\/g, "\\\\").replace(/\"/g, '\\\"').replace(/'/g, "\\'")}"` : raw;
      let parsed: string;
      try {
        parsed = JSON.parse(normalizedRaw);
      } catch (error: unknown) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Invalid string literal at position ${i}: ${message}`);
      }

      tokens.push({ type: "string", value: parsed, position: i });
      i = j + 1;
      continue;
    }

    if (/[0-9]/.test(ch)) {
      let j = i + 1;
      while (j < input.length && /[0-9.]/.test(input[j] ?? "")) {
        j += 1;
      }
      const raw = input.slice(i, j);
      if (!/^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$/.test(raw)) {
        throw new Error(`Invalid number at position ${i}`);
      }
      tokens.push({ type: "number", value: raw, position: i });
      i = j;
      continue;
    }

    if (/[A-Za-z_]/.test(ch)) {
      let j = i + 1;
      while (j < input.length && /[A-Za-z0-9_]/.test(input[j] ?? "")) {
        j += 1;
      }
      const ident = input.slice(i, j);
      if (ident === "true" || ident === "false") {
        tokens.push({ type: "boolean", value: ident, position: i });
      } else if (ident === "null") {
        tokens.push({ type: "null", value: ident, position: i });
      } else if (ident === "in") {
        tokens.push({ type: "operator", value: ident, position: i });
      } else {
        tokens.push({ type: "identifier", value: ident, position: i });
      }
      i = j;
      continue;
    }

    throw new Error(`Unexpected token "${ch}" at position ${i}`);
  }

  return tokens;
}

function peek(state: ParserState): Token | null {
  return state.tokens[state.index] ?? null;
}

function consume(state: ParserState): Token | null {
  const token = state.tokens[state.index] ?? null;
  if (token) {
    state.index += 1;
  }
  return token;
}

function expect(state: ParserState, type: TokenType, value = ""): Token {
  const token = consume(state);
  if (!token || token.type !== type || (value && token.value !== value)) {
    const received = token ? `${token.type}:${token.value}` : "<eof>";
    const suffix = value ? ` ${value}` : "";
    throw new Error(`Expected ${type}${suffix}, got ${received}`);
  }
  return token;
}

function parseExpression(state: ParserState): ASTNode {
  return parseOr(state);
}

function parseOr(state: ParserState): ASTNode {
  let node = parseAnd(state);
  while (true) {
    const next = peek(state);
    if (!next || next.type !== "operator" || next.value !== "||") {
      return node;
    }
    consume(state);
    node = {
      type: "binary",
      operator: "||",
      left: node,
      right: parseAnd(state),
    };
  }
}

function parseAnd(state: ParserState): ASTNode {
  let node = parseComparison(state);
  while (true) {
    const next = peek(state);
    if (!next || next.type !== "operator" || next.value !== "&&") {
      return node;
    }
    consume(state);
    node = {
      type: "binary",
      operator: "&&",
      left: node,
      right: parseComparison(state),
    };
  }
}

function parseComparison(state: ParserState): ASTNode {
  let node = parsePrimary(state);
  while (true) {
    const next = peek(state);
    if (!next || next.type !== "operator") {
      return node;
    }

    if (!["==", "!=", ">", ">=", "<", "<=", "in"].includes(next.value)) {
      return node;
    }

    const operator = next.value;
    consume(state);
    node = {
      type: "binary",
      operator,
      left: node,
      right: parsePrimary(state),
    };
  }
}

function parsePrimary(state: ParserState): ASTNode {
  const token = peek(state);
  if (!token) {
    throw new Error("Unexpected end of expression");
  }

  if (token.type === "paren_open") {
    consume(state);
    const expr = parseExpression(state);
    expect(state, "paren_close");
    return expr;
  }

  if (token.type === "string") {
    consume(state);
    return { type: "literal", value: token.value };
  }

  if (token.type === "number") {
    consume(state);
    return { type: "literal", value: Number(token.value) };
  }

  if (token.type === "boolean") {
    consume(state);
    return { type: "literal", value: token.value === "true" };
  }

  if (token.type === "null") {
    consume(state);
    return { type: "literal", value: null };
  }

  if (token.type === "identifier") {
    return parsePath(state);
  }

  throw new Error(`Unexpected token ${token.type}:${token.value} at position ${token.position}`);
}

function parsePath(state: ParserState): ASTNode {
  const first = expect(state, "identifier").value;
  const segments = [first];

  while (true) {
    const dot = peek(state);
    if (!dot || dot.type !== "dot") {
      break;
    }
    consume(state);
    const segment = expect(state, "identifier").value;
    segments.push(segment);
  }

  return {
    type: "path",
    segments,
  };
}

function resolvePath(scope: PipelineConditionScope, segments: string[]): JSONValue {
  let current: unknown = scope;
  for (const segment of segments) {
    if (current === null || typeof current !== "object" || Array.isArray(current)) {
      return null;
    }
    current = (current as Record<string, unknown>)[segment];
  }

  if (current === undefined) {
    return null;
  }

  if (
    current === null ||
    typeof current === "string" ||
    typeof current === "number" ||
    typeof current === "boolean" ||
    Array.isArray(current) ||
    (typeof current === "object" && current !== null)
  ) {
    return current as JSONValue;
  }

  return null;
}

function asNumber(value: JSONValue): number | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  return null;
}

function stringifyComparable(value: JSONValue): string {
  if (value === null) {
    return "null";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}

function isEqual(left: JSONValue, right: JSONValue): boolean {
  if (typeof left === "number" && typeof right === "number") {
    return left === right;
  }
  if (typeof left === "string" && typeof right === "string") {
    return left === right;
  }
  if (typeof left === "boolean" && typeof right === "boolean") {
    return left === right;
  }
  if (left === null || right === null) {
    return left === right;
  }
  return JSON.stringify(left) === JSON.stringify(right);
}

function isTruthy(value: JSONValue): boolean {
  if (value === null) {
    return false;
  }
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  if (typeof value === "string") {
    return value.length > 0;
  }
  if (Array.isArray(value)) {
    return value.length > 0;
  }
  return Object.keys(value).length > 0;
}

function evaluateNode(node: ASTNode, scope: PipelineConditionScope): JSONValue {
  if (node.type === "literal") {
    return node.value;
  }
  if (node.type === "path") {
    return resolvePath(scope, node.segments);
  }

  const left = evaluateNode(node.left, scope);

  switch (node.operator) {
    case "&&": {
      if (!isTruthy(left)) {
        return false;
      }
      const right = evaluateNode(node.right, scope);
      return isTruthy(right);
    }
    case "||": {
      if (isTruthy(left)) {
        return true;
      }
      const right = evaluateNode(node.right, scope);
      return isTruthy(right);
    }
    default:
      break;
  }

  const right = evaluateNode(node.right, scope);
  switch (node.operator) {
    case "==":
      return isEqual(left, right);
    case "!=":
      return !isEqual(left, right);
    case ">": {
      const leftNumber = asNumber(left);
      const rightNumber = asNumber(right);
      if (leftNumber !== null && rightNumber !== null) {
        return leftNumber > rightNumber;
      }
      return stringifyComparable(left) > stringifyComparable(right);
    }
    case ">=": {
      const leftNumber = asNumber(left);
      const rightNumber = asNumber(right);
      if (leftNumber !== null && rightNumber !== null) {
        return leftNumber >= rightNumber;
      }
      return stringifyComparable(left) >= stringifyComparable(right);
    }
    case "<": {
      const leftNumber = asNumber(left);
      const rightNumber = asNumber(right);
      if (leftNumber !== null && rightNumber !== null) {
        return leftNumber < rightNumber;
      }
      return stringifyComparable(left) < stringifyComparable(right);
    }
    case "<=": {
      const leftNumber = asNumber(left);
      const rightNumber = asNumber(right);
      if (leftNumber !== null && rightNumber !== null) {
        return leftNumber <= rightNumber;
      }
      return stringifyComparable(left) <= stringifyComparable(right);
    }
    case "in": {
      if (Array.isArray(right)) {
        return right.some((candidate) => isEqual(left, candidate));
      }
      if (typeof right === "string") {
        return right.includes(stringifyComparable(left));
      }
      return false;
    }
    default:
      throw new Error(`Unsupported operator: ${node.operator}`);
  }
}

export interface CompiledCondition {
  raw: string;
  ast: ASTNode;
}

export function compileCondition(condition: string): CompiledCondition {
  const raw = String(condition ?? "").trim();
  if (!raw) {
    throw new Error("Condition must not be empty");
  }

  const tokens = tokenize(raw);
  const state: ParserState = {
    tokens,
    index: 0,
  };
  const ast = parseExpression(state);
  if (state.index !== tokens.length) {
    const next = tokens[state.index];
    const value = next ? `${next.type}:${next.value}` : "<eof>";
    throw new Error(`Unexpected token after expression: ${value}`);
  }

  return { raw, ast };
}

export function evaluateCondition(compiled: CompiledCondition, scope: PipelineConditionScope): boolean {
  const value = evaluateNode(compiled.ast, scope);
  return isTruthy(value);
}

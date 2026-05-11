import type { Rule } from "eslint";

const colorNames = [
  "slate",
  "gray",
  "zinc",
  "neutral",
  "stone",
  "red",
  "orange",
  "amber",
  "yellow",
  "lime",
  "green",
  "emerald",
  "teal",
  "cyan",
  "sky",
  "blue",
  "indigo",
  "violet",
  "purple",
  "fuchsia",
  "pink",
  "rose",
].join("|");

const colorUtilities = [
  "bg",
  "text",
  "border",
  "border-x",
  "border-y",
  "border-s",
  "border-e",
  "border-t",
  "border-r",
  "border-b",
  "border-l",
  "ring",
  "ring-offset",
  "divide",
  "outline",
  "decoration",
  "accent",
  "caret",
  "fill",
  "stroke",
  "placeholder",
  "shadow",
  "from",
  "via",
  "to",
].join("|");

const rawColorUtilityPattern = new RegExp(
  `^(?:${colorUtilities})-(?:${colorNames})-\\d+(?:\\/\\d+)?$`,
);
const rawArbitraryColorPattern = new RegExp(
  `^(?:${colorUtilities})-\\[(?:#[0-9a-fA-F]{3,8}|rgba?\\(.+\\))\\]$`,
);
const rawRadiusUtilityPattern =
  /^rounded(?:-[trbl](?:-[lr])?)?-(?:none|sm|md|lg|xl|2xl|3xl|full)$/;
const rawArbitraryRadiusPattern = /^rounded(?:-[trbl](?:-[lr])?)?-\[(?!var\(--).+\]$/;
const rawSpacingUtilityPattern = /^-?(?:m[trblxy]?|p[trblxy]?|gap(?:-[xy])?|space-[xy])-\d+(?:\.\d+)?$/;
const rawArbitrarySpacingPattern =
  /^-?(?:m[trblxy]?|p[trblxy]?|gap(?:-[xy])?|space-[xy])-\[(?!var\(--).+\]$/;
const rawStyleColorPattern = /#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})\b|rgba?\([^)]*\)/;

type TemplateQuasi = { value: { cooked?: string | null } };
type AttributeValue =
  | { type: "Literal"; value: unknown }
  | { type: "JSXExpressionContainer"; expression: Expression };
type Expression =
  | { type: "Literal"; value: unknown }
  | {
      type: "TemplateLiteral";
      expressions: unknown[];
      quasis: TemplateQuasi[];
    }
  | {
      type: "ObjectExpression";
      properties: Array<{
        type: string;
        value?: Expression;
      }>;
    };
type JSXAttributeNode = {
  name: { type: string; name?: string };
  value?: AttributeValue | null;
};

function baseUtility(token: string): string {
  let bracketDepth = 0;
  let lastVariantSeparator = -1;

  for (let index = 0; index < token.length; index += 1) {
    if (token[index] === "[") {
      bracketDepth += 1;
    } else if (token[index] === "]") {
      bracketDepth = Math.max(0, bracketDepth - 1);
    } else if (token[index] === ":" && bracketDepth === 0) {
      lastVariantSeparator = index;
    }
  }

  const utility = lastVariantSeparator === -1 ? token : token.slice(lastVariantSeparator + 1);
  return utility.startsWith("!") ? utility.slice(1) : utility;
}

function isRawToken(token: string): boolean {
  const utility = baseUtility(token);
  return (
    rawColorUtilityPattern.test(utility) ||
    rawArbitraryColorPattern.test(utility) ||
    rawRadiusUtilityPattern.test(utility) ||
    rawArbitraryRadiusPattern.test(utility) ||
    rawSpacingUtilityPattern.test(utility) ||
    rawArbitrarySpacingPattern.test(utility)
  );
}

function rawTokens(value: string): string[] {
  return value.split(/\s+/).filter((token) => token.length > 0 && isRawToken(token));
}

function staticString(expression: Expression): string | null {
  if (expression.type === "Literal" && typeof expression.value === "string") {
    return expression.value;
  }
  if (expression.type === "TemplateLiteral" && expression.expressions.length === 0) {
    return expression.quasis.map((quasi: TemplateQuasi) => quasi.value.cooked ?? "").join("");
  }
  return null;
}

function expressionStyleColorLiterals(expression: Expression): string[] {
  if (expression.type !== "ObjectExpression") {
    const value = staticString(expression);
    return value && rawStyleColorPattern.test(value) ? [value] : [];
  }

  return expression.properties.flatMap((property) => {
    if (property.type !== "Property" || !property.value) {
      return [];
    }

    const value = staticString(property.value);
    return value && rawStyleColorPattern.test(value) ? [value] : [];
  });
}

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "disallow raw color, radius, and spacing utility classes",
    },
    messages: {
      rawToken: "Use design tokens instead of raw utility class '{{ token }}'.",
      rawStyleColor: "Use a CSS custom property token instead of raw style color '{{ color }}'.",
    },
    schema: [],
  },
  create(context) {
    return {
      JSXAttribute(node: JSXAttributeNode) {
        if (node.name.type !== "JSXIdentifier" || !node.value) {
          return;
        }

        if (node.name.name === "className") {
          const value =
            node.value.type === "Literal"
              ? staticString(node.value)
              : node.value.type === "JSXExpressionContainer"
                ? staticString(node.value.expression)
                : null;

          if (!value) {
            return;
          }

          for (const token of rawTokens(value)) {
            context.report({ node: node.value, messageId: "rawToken", data: { token } });
          }
        }

        if (node.name.name === "style") {
          const colors =
            node.value.type === "Literal"
              ? expressionStyleColorLiterals(node.value)
              : node.value.type === "JSXExpressionContainer"
                ? expressionStyleColorLiterals(node.value.expression)
                : [];

          for (const color of colors) {
            context.report({ node: node.value, messageId: "rawStyleColor", data: { color } });
          }
        }
      },
    };
  },
};

export default rule;

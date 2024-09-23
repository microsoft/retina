import { PrismTheme } from "prism-react-renderer";

// Modified version of github theme but with a11y colors
export const githubA11yLight: PrismTheme = {
  plain: { color: "#393A34", backgroundColor: "#f6f8fa" },
  styles: [
    {
      types: ["comment", "prolog", "doctype", "cdata"],
      style: { color: "#6e6e61" },
    },
    { types: ["namespace"], style: { opacity: 0.7 } },
    { types: ["string", "attr-value"], style: { color: "#d71066" } },
    { types: ["punctuation", "operator"], style: { color: "#393A34" } },
    {
      types: [
        "entity",
        "url",
        "symbol",
        "number",
        "boolean",
        "variable",
        "constant",
        "property",
        "regex",
        "inserted",
      ],
      style: { color: "#257c7a" },
    },
    {
      types: ["atrule", "keyword", "attr-name", "selector"],
      style: { color: "#00769f" },
    },
    { types: ["function", "deleted", "tag"], style: { color: "#cc3745" } },
    { types: ["function-variable"], style: { color: "#6f42c1" } },
    { types: ["tag", "selector", "keyword"], style: { color: "#00009f" } },
  ],
};

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Statement } from "./Statement";

const markdown = `# A + B

Read two integers \`a\` and \`b\`. Print **a + b**.

## Constraints

- |a|, |b| ≤ 10^18

| n | answer |
|---|---|
| 1 | 2 |

\`\`\`
1 2
\`\`\`

See [the rules](https://example.com/rules).
`;

describe("Statement", () => {
  it("renders Markdown below the page title", () => {
    render(<Statement markdown={markdown} />);

    // The leading title is the page heading already.
    expect(screen.queryByRole("heading", { name: "A + B" })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Constraints" })).toBeInTheDocument();
    expect(screen.getByText("a", { selector: "code" })).toBeInTheDocument();
    expect(screen.getByText("a + b", { selector: "strong" })).toBeInTheDocument();
    expect(screen.getByRole("listitem")).toHaveTextContent("|a|, |b| ≤ 10^18");
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByText("1 2", { selector: "pre code" })).toBeInTheDocument();

    const link = screen.getByRole("link", { name: "the rules" });
    expect(link).toHaveAttribute("href", "https://example.com/rules");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("keeps lower headings and turns a later level-1 heading into level 2", () => {
    render(<Statement markdown={"Intro.\n\n# Input\n\n### Notes"} />);

    expect(screen.getByRole("heading", { level: 2, name: "Input" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "Notes" })).toBeInTheDocument();
  });

  it("does not render raw HTML, images or script URLs", () => {
    const hostile = [
      "<script>alert(1)</script>",
      '<img src="x" onerror="alert(1)">',
      "![tracker](https://example.com/pixel.png)",
      "[click](javascript:alert(1))",
    ].join("\n\n");
    const { container } = render(<Statement markdown={hostile} />);

    expect(container.querySelector("script, img, [onerror]")).toBeNull();
    const link = screen.getByText("click");
    expect(link.getAttribute("href") ?? "").not.toMatch(/javascript:/i);
  });
});

import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

// The page heading is the problem title. Statements start with the same title as a
// level-1 heading, so it is left out, and other headings start at level 2.
const LEADING_TITLE = /^\s*#[ \t]+[^\n]*\n*/;

const components: Components = {
  h1: ({ children }) => <h2>{children}</h2>,
  // Links in a statement leave the site in a new tab and do not pass the referrer.
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  ),
};

/**
 * Renders a problem statement (GitHub-flavored Markdown). Raw HTML in the source is
 * dropped, not rendered, and react-markdown removes unsafe URLs such as javascript:.
 */
export function Statement({ markdown }: { markdown: string }) {
  return (
    <div className="statement">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components} skipHtml disallowedElements={["img"]}>
        {markdown.replace(LEADING_TITLE, "")}
      </ReactMarkdown>
    </div>
  );
}

import Editor, { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";
import EditorWorker from "monaco-editor/editor/editor.worker?worker";
import { useEffect, useRef } from "react";
import { config } from "../config";

// Bundle Monaco instead of loading it from a CDN at runtime: no third-party script,
// works offline and with a strict Content-Security-Policy.
self.MonacoEnvironment = { getWorker: () => new EditorWorker() };
// JavaScript keeps its syntax highlighting but not the TypeScript language service: that
// worker is about 7 MB, and the judge reports syntax errors anyway (node --check).
monaco.typescript.javascriptDefaults.setModeConfiguration({
  completionItems: false,
  hovers: false,
  documentSymbols: false,
  definitions: false,
  references: false,
  documentHighlights: false,
  rename: false,
  diagnostics: false,
  documentRangeFormattingEdits: false,
  signatureHelp: false,
  onTypeFormattingEdits: false,
  codeActions: false,
  inlayHints: false,
});
loader.config({ monaco });

// The editor sits on the board: same black, flap-white text, amber caret and selection.
const THEME = "codearena-board";
monaco.editor.defineTheme(THEME, {
  base: "vs-dark",
  inherit: true,
  rules: [
    { token: "comment", foreground: "8b919a", fontStyle: "italic" },
    { token: "keyword", foreground: "f2c46d" },
    { token: "string", foreground: "a9d6a0" },
    { token: "number", foreground: "eba982" },
    { token: "type", foreground: "9fc4e8" },
  ],
  colors: {
    "editor.background": "#0e1013",
    "editor.foreground": "#ecebe6",
    "editorLineNumber.foreground": "#868c95",
    "editorLineNumber.activeForeground": "#ecebe6",
    "editorCursor.foreground": "#f2b134",
    "editor.selectionBackground": "#f2b13447",
    "editor.inactiveSelectionBackground": "#f2b13426",
    "editor.lineHighlightBackground": "#15171a",
    "editor.lineHighlightBorder": "#15171a",
    "editorIndentGuide.background1": "#262a2f",
    "editorWhitespace.foreground": "#3a3f46",
    "scrollbarSlider.background": "#3a3f4699",
    "scrollbarSlider.hoverBackground": "#4a5058",
    "scrollbarSlider.activeBackground": "#9ba1aa66",
    "scrollbar.shadow": "#00000000",
    "editorOverviewRuler.border": "#00000000",
    "editorOverviewRuler.background": "#0e1013",
    "editorWidget.background": "#1c1f23",
    "editorWidget.border": "#3a3f46",
  },
});

interface Props {
  value: string;
  onChange: (value: string) => void;
  label: string;
  /** Monaco language id, for example python or cpp. */
  language: string;
}

/**
 * window.__codeArenaEditor holds the editor instance, only in development auth mode (local stack and
 * the docker compose E2E tests; deployed builds always use Cognito). Browser tests set the
 * source through it: typed or inserted text is altered differently by each browser (auto-close,
 * auto-indent, duplicated input events in Firefox).
 */
type TestWindow = Window & { __codeArenaEditor?: monaco.editor.IStandaloneCodeEditor };

export function CodeEditor({ value, onChange, label, language }: Props) {
  const mounted = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  useEffect(
    () => () => {
      const w = window as TestWindow;
      if (mounted.current && w.__codeArenaEditor === mounted.current) delete w.__codeArenaEditor;
    },
    [],
  );

  return (
    <div className="editor" data-testid="code-editor">
      <Editor
        height="100%"
        language={language}
        theme={THEME}
        value={value}
        onChange={(v) => onChange(v ?? "")}
        onMount={(editor) => {
          mounted.current = editor;
          // The code face is self-hosted and may arrive after Monaco measured its glyphs.
          void document.fonts.ready.then(() => monaco.editor.remeasureFonts());
          if (config.authMode === "dev") (window as TestWindow).__codeArenaEditor = editor;
        }}
        options={{
          minimap: { enabled: false },
          fontSize: 14,
          lineHeight: 22,
          fontFamily: "'JetBrains Mono', ui-monospace, Consolas, monospace",
          tabSize: 4,
          ariaLabel: label,
          automaticLayout: true,
          scrollBeyondLastLine: false,
          padding: { top: 16, bottom: 16 },
          renderLineHighlight: "line",
          cursorBlinking: "solid",
          // The board has no ruler: no overview marks, a thin scrollbar from the ramp.
          overviewRulerLanes: 0,
          overviewRulerBorder: false,
          hideCursorInOverviewRuler: true,
          scrollbar: { verticalScrollbarSize: 10, horizontalScrollbarSize: 10, useShadows: false },
        }}
      />
    </div>
  );
}

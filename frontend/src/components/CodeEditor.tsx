import Editor, { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";
import EditorWorker from "monaco-editor/editor/editor.worker?worker";
import { useEffect, useRef } from "react";
import { config } from "../config";

// Bundle Monaco instead of loading it from a CDN at runtime: no third-party script,
// works offline and with a strict Content-Security-Policy.
self.MonacoEnvironment = { getWorker: () => new EditorWorker() };
loader.config({ monaco });

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
        height="320px"
        language={language}
        theme="vs-dark"
        value={value}
        onChange={(v) => onChange(v ?? "")}
        onMount={(editor) => {
          mounted.current = editor;
          if (config.authMode === "dev") (window as TestWindow).__codeArenaEditor = editor;
        }}
        options={{ minimap: { enabled: false }, fontSize: 14, tabSize: 4, ariaLabel: label }}
      />
    </div>
  );
}

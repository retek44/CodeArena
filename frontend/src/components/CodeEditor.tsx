import Editor, { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";
import EditorWorker from "monaco-editor/editor/editor.worker?worker";

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

export function CodeEditor({ value, onChange, label, language }: Props) {
  return (
    <div className="editor" data-testid="code-editor">
      <Editor
        height="320px"
        language={language}
        theme="vs-dark"
        value={value}
        onChange={(v) => onChange(v ?? "")}
        options={{ minimap: { enabled: false }, fontSize: 14, tabSize: 4, ariaLabel: label }}
      />
    </div>
  );
}

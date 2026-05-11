import MonacoEditor from "@monaco-editor/react";
import styles from "./MobilePklViewer.module.css";

interface Props {
  source: string;
  path: string;
}

export function MobilePklViewer({ source, path }: Props) {
  return (
    <div className={styles.page}>
      <div className={styles.topBar}>
        <span className={styles.path}>{path}</span>
        <span className={styles.readOnly}>Read-only</span>
      </div>
      <div className={styles.editorWrap}>
        <MonacoEditor
          height="100%"
          language="pkl"
          value={source}
          options={{
            readOnly: true,
            minimap: { enabled: false },
            fontSize: 13,
            lineNumbers: "on",
            wordWrap: "on",
            scrollBeyondLastLine: false,
          }}
          theme="vs-dark"
        />
      </div>
    </div>
  );
}

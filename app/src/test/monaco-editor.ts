export const languages = {
  CompletionItemKind: {
    Function: 1,
    Variable: 2,
    Keyword: 3,
    Text: 4,
  },
  registerCompletionItemProvider: () => ({ dispose: () => {} }),
  registerHoverProvider: () => ({ dispose: () => {} }),
  registerDefinitionProvider: () => ({ dispose: () => {} }),
  registerDocumentSemanticTokensProvider: () => ({ dispose: () => {} }),
};

export const editor = {
  getModels: () => [],
  onDidCreateModel: () => ({ dispose: () => {} }),
  setModelMarkers: () => {},
};

export const MarkerSeverity = {
  Error: 8,
  Warning: 4,
};

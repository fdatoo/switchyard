import { create } from "zustand";
type EditorState = { dirty: boolean; setDirty: (v: boolean) => void };
export const useEditorStore = create<EditorState>((set) => ({ dirty: false, setDirty: (v) => set({ dirty: v }) }));

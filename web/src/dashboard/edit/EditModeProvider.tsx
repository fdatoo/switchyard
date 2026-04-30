import { createContext, useContext, useState, type ReactNode } from "react";
type Ctx = { editing: boolean; toggleEdit: () => void };
const Ctx = createContext<Ctx>({ editing: false, toggleEdit: () => {} });
export function EditModeProvider({ children }: { children: ReactNode }) {
  const [editing, setEditing] = useState(false);
  return <Ctx.Provider value={{ editing, toggleEdit: () => setEditing(e => !e) }}>{children}</Ctx.Provider>;
}
export const useEditMode = () => useContext(Ctx);

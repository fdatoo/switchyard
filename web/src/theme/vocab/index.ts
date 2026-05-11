import { useContext } from "react";
import { LanguageContext } from "../language-provider";
import { friendlyVocab } from "./friendly";
import { developerVocab } from "./developer";
import { ambientVocab } from "./ambient";

export type RouteId =
  | "home"
  | "rooms"
  | "activity"
  | "automations"
  | "devices"
  | "settings";

type VocabMap = Record<RouteId, string>;

const vocabByLanguage: Record<"friendly" | "developer" | "ambient", VocabMap> = {
  friendly: friendlyVocab,
  developer: developerVocab,
  ambient: ambientVocab,
};

export interface VocabHandle {
  label: (routeId: RouteId) => string;
}

export function useVocab(): VocabHandle {
  const ctx = useContext(LanguageContext);
  // Fall back to friendly when rendered outside a LanguageProvider
  const language = ctx?.language ?? "friendly";
  const map = vocabByLanguage[language];
  return {
    label: (routeId) => map[routeId],
  };
}

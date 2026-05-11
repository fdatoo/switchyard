import { useLanguage } from "../language-provider";
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

// eslint-disable-next-line react-refresh/only-export-components
export function useVocab(): VocabHandle {
  const { language } = useLanguage();
  const map = vocabByLanguage[language];
  return {
    label: (routeId) => map[routeId],
  };
}

import type { ComponentType } from "react";
import type { WidgetProps } from "@gohome/widget-sdk";

const cache = new Map<string, Promise<Record<string, ComponentType<WidgetProps>>>>();

export async function loadPack(packName: string, version: string, hash: string): Promise<Record<string, ComponentType<WidgetProps>>> {
  const key = `${packName}-${version}-${hash}`;
  let p = cache.get(key);
  if (!p) {
    p = import(/* @vite-ignore */ `/widgets/${packName}/${version}/bundle.js?h=${hash}`).then(
      (m: Record<string, ComponentType<WidgetProps>>) => m
    );
    cache.set(key, p);
  }
  return p;
}

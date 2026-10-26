import { Fragment, createElement } from "react";
import type { Composition, SlotName } from "@/shared/composition";
import { frontendModules } from "@/app/modules";

// Each module's slot content is keyed by the module, so a slot renders as a proper list.
const slots: Composition["slots"] = {};
for (const m of frontendModules) {
  for (const [name, node] of Object.entries(m.slots ?? {})) {
    (slots[name as SlotName] ??= []).push(createElement(Fragment, { key: m.id }, node));
  }
}

/** Providers modules contribute, outermost first. */
export const moduleProviders = frontendModules.flatMap((m) => (m.provider ? [m.provider] : []));

/** Navigation and slot content gathered from every registered module. */
export const composition: Composition = {
  nav: frontendModules
    .flatMap((m) => m.nav ?? [])
    .sort((a, b) => (a.order ?? 0) - (b.order ?? 0)),
  slots,
};

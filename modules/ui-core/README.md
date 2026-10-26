# ui-core

The foundation kit, in Bingkai Navy's component design (mini-erp-v2's visual design, rewritten
generic). Installed into `apps/web/src/shared/`; nothing domain-specific, every text overridable,
Indonesian by default. Needs kernel 3 (the scaffold's restyled `button`, `input`, `card`,
`dialog` and `shared/lib/field-classes.ts`).

**Primitives** (`shared/components/primitives/`, shadcn `new-york`, restyled to the tokens):
alert, alert-dialog, avatar, breadcrumb, checkbox, collapsible, dropdown-menu, kbd,
native-select, popover, progress, radio-group, scroll-area, separator, sheet, switch, table,
tabs, textarea, toggle, toggle-group, tooltip. Every field control uses `FIELD` (36px, card
background, soft shadow, hover border, soft focus ring); overlays use `--shadow-pop`; tables have
a quiet header on `--background` and rows padded by `--row-py`.

**Components** (`shared/components/ui/`):

| Component | Use |
|---|---|
| `Page`, `PageColumns` | Page root (24px rhythm); two columns for a record page |
| `PageHeader` | The page's one `<h1>`, meta, subtitle, description, actions, band |
| `SectionCard` | Titled, border-only card (or plain section) |
| `SummaryCard` | KPI: label, big tabular number, hint or delta; clickable |
| `StatusBadge` | Status pill with a dot; tone = `success · warning · danger · info · neutral · muted` |
| `Notice` | Inline message box by tone |
| `EmptyState` · `ErrorState` · `NotFoundState` | Empty (with next action), failed load ("Coba lagi"), 404 |
| `TableSkeleton` · `CardSkeleton` · `DetailSkeleton` | Loading shaped like the result |
| `DescriptionList` | Label–value grid, "–" for empty |
| `Money` · `Qty` · `DocNumber` | Rupiah (accounting mode), quantities, copyable codes |
| `Modal` · `FormSheet` | Dialog for ≤ 6 fields; right panel for master data and record actions |

**Lib:** `shared/lib/format.ts` (`formatIDR`, `formatNumber`, `formatDate`, `formatDateTime`,
`todayISO`), `shared/lib/tone.ts` (`Tone`).

## Changes

- **0.3.0** — `shared/hooks/use-resource.ts`: `useResource(load, deps)` loads what a page shows,
  aborts the previous request when its inputs change, and drops late answers, so an old search
  never replaces a new one. Use it for every list and detail page.

- **0.2.0** — needs kernel 3. Bingkai Navy component design; adds primitives (popover,
  native-select, radio-group, switch, tabs, …) and the components above. `DataTable` and
  pagination moved to `ui-data`, `FormField` to `ui-forms`, `ConfirmDialog` to `ui-feedback` —
  add those kits when upgrading (`module.mjs add` takes the unchanged files over).

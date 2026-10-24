# app-shell

The `layout` of the frontend registry: every protected page renders inside it.

- **Sidebar** — shadcn `Sidebar` (`variant="inset"`, `collapsible="icon"`, Ctrl/Cmd+B), built from
  the `nav` entries every registered module contributes, grouped by `group` and sorted by
  `order`. The active entry (longest matching path) is a pill. Becomes an off-canvas sheet below
  768px.
- **Header** — 48px, sticky inside the canvas: sidebar toggle and a breadcrumb of the active
  entry's group and label. The page's own `<h1>` stays in its `PageHeader`.
- **Slots** — renders whatever modules put in `"sidebar-footer"` (e.g. auth's user menu) and
  `"header-end"`.
- Entries with a `permission` show only to users who hold it (`useAccess` from
  `@/shared/access`).
- The app name in the sidebar comes from `<title>` in `index.html`.

It imports no module: navigation and slots arrive through `@/shared/composition`. It looks like
Bingkai Navy under that preset and stays neutral without it — only semantic `sidebar-*` tokens.

Installs `shared/components/primitives/sidebar.tsx` and `shared/hooks/use-mobile.ts` (shadcn),
and builds on `ui-core` (breadcrumb, separator, sheet, tooltip).

## Changes

- **0.2.0** — needs kernel 2. Hides nav entries whose `permission` the user does not hold.
- **0.3.0** — needs kernel 3 (with ui-core 0.2.0). Indonesian accessible labels.

# ui-core

Shared UI kit, installed into `apps/web/src/shared/`. Nothing is registered: modules import it.

**Primitives** (`shared/components/primitives/`, shadcn `new-york` as the CLI writes them):
alert, alert-dialog, avatar, breadcrumb, checkbox, collapsible, dropdown-menu, separator, sheet,
table, textarea, tooltip. Two deviations from the CLI output, both to honor the token contract:
overlays use `bg-foreground/50` instead of `bg-black/50`, and table cells take their vertical
padding from `--row-py` (density).

**Building blocks** (`shared/components/ui/`):

| Component | Use |
|---|---|
| `PageHeader` | The page's one `<h1>`, description, actions (main action last) |
| `DataTable` | Columns + rows, sticky header, skeleton while loading, empty state, clickable rows (keyboard too) |
| `PaginationBar` | "1–25 of 128" + previous/next, from the API's `meta` |
| `FormField` | Label + control + hint/error, ids and `aria-*` wired |
| `EmptyState` | Nothing to show — with the next action |
| `ErrorState` | A failed load with the API's message and "Try again" |
| `ConfirmDialog` | Confirmation before a hard-to-undo action; async-aware |

Built only on the scaffold's primitives, `cn`, `lucide-react`, and `@/shared/types/api`.

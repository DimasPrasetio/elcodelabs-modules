# ui-data

Lists and tables, in Bingkai Navy's design. Builds on `ui-core`.

| Component | Use |
|---|---|
| `DataTable` | Columns + rows; sticky header; server sort on header click; whole-row click (Enter too); selection with "n dipilih" bar and bulk actions; "Kolom" chooser remembered per table (`columnsKey`); skeleton, empty, and error states built in |
| `Pagination` | "1–25 dari 128 data", rows per page, Sebelumnya/Berikutnya — takes the API's `meta` as is |
| `Toolbar` | Search + up to 3 filters, more in a "Filter" popover, active filters as removable chips, actions on the right |
| `FilterBar` | A card of filter fields for report pages |
| `SearchInput` | Debounced search (reports settled text only) |
| `ActionMenu` | "⋯" menu for secondary actions; destructive ones red and last |
| `NavTabs`, `useUrlTab` | Tabs as links, or a tab kept in `?tab=` |

The list template: `PageHeader` (one primary action) → card with `Toolbar plain` + `DataTable` +
`Pagination`.

## Changes

- **0.2.0** — a clickable row shows a visible focus ring when reached with the keyboard
  (WCAG 2.4.7).

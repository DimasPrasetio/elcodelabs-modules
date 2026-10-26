# Rule: API standard

Applies to: `apps/api/**`.

- Version endpoints under `/api/v1`.
- Success: `{ success: true, message, data }`. Error: `{ success: false, message, errors }`.
- Paginated: include `meta: { page, limit, total, total_pages }`.

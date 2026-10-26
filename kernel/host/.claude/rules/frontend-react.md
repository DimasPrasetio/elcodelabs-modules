# Rule: Frontend (React)

Applies to: `apps/web/**`.

- React 19 + Tailwind 4, react-router-dom with lazy-loaded routes/pages.
- State: Zustand (global in `shared/stores`, module state in `modules/<m>/stores`).
- Validation: Zod schemas in `modules/<m>/schemas`. Prefer `z.infer` over manual form types.
- HTTP: all requests go through `shared/services/http-client.ts` (one Axios instance).
- Use the `@/*` import alias.
- UI foundation is shadcn/ui on Radix: primitives in `shared/components/primitives` (add more with
  `npx shadcn@latest add <name>`), generic compositions in `shared/components/ui`, page patterns
  in `shared/components/patterns`. Merge classes with `cn` from the `cn` package.
- Before writing a generic component, check the library UI kits (ui-core, ui-data, ui-forms,
  ui-feedback — see the module-library catalog); field-like controls use `FIELD` from
  `shared/lib/field-classes.ts`. User-facing copy is Indonesian unless the project says otherwise.
- Color, type, radius, and elevation come only from `src/theme/theme.css`, through semantic
  classes (`bg-card`, `text-muted-foreground`, `border-border`, `text-destructive`, …). No raw
  colors in components (hex, `bg-blue-600`, `text-[#…]`). Token names are a contract: change
  their values, never rename or remove one.
- Before building or reshaping UI, read `knowledge/DESIGN.md`. If it is not researched yet, is
  older than ~6 months, or doesn't cover the screen type at hand, first research current sources
  on the internet (real products of this app type, a fitting design system, UX research) and
  record findings, direction, and dated sources there. Extract patterns — never copy another
  product's branding, assets, or exact layout.
- Shared UI components must be domain-agnostic. Use the `frontend-design` skill for UI; if it isn't installed, install it first from https://github.com/anthropics/skills/tree/main/skills/frontend-design (do not skip it).

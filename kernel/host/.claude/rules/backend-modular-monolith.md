# Rule: Backend modular monolith

Applies to: `apps/api/**`.

- Each module is a boundary. It offers `contracts/` (owned by the provider) and declares what it
  needs as ports in its own `ports/` (owned by the consumer).
- Forbidden: importing another module at all — its internals and its `contracts/` alike. Only
  the composition root imports modules, connecting a provider's client to a consumer's port
  with a small adapter that holds no business logic.
- Forbidden: cross-module DB joins and cross-module foreign keys.
- Cross-module relations are primitive IDs; resolve them through the module's own ports.
- A change spanning modules commits atomically through the kernel's `d.Tx` (ports called with its ctx);
  reactions that may happen later go through `d.Events` — never a transaction a module opens itself.
- `shared/` holds only technical utilities (errors, response, request, logger, validator, pagination, utils) — never domain logic.

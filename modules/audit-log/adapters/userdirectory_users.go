package modules

import (
	"context"

	auditports "__GO_MODULE__/internal/modules/audit-log/ports"
	userscontracts "__GO_MODULE__/internal/modules/users/contracts"
)

var _ auditports.UserDirectory = auditUserDirectory{}

// auditUserDirectory satisfies audit-log's UserDirectory port with the users module's client.
// Mapping only: an ID the users module no longer knows is left out.
type auditUserDirectory struct {
	users userscontracts.UserClient
}

func (a auditUserDirectory) Names(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		if u, err := a.users.GetByID(ctx, id); err == nil {
			out[id] = u.Name
		}
	}
	return out, nil
}

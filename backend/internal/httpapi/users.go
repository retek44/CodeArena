package httpapi

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"

	"entgo.io/ent/dialect/sql"
)

// ensureUser creates the user's row on their first write. ON CONFLICT DO NOTHING has
// no conflict target on purpose: two concurrent first requests of a new user (two
// tabs, or the C++ and Python submissions of the e2e test) also collide on the unique
// username index, and ON CONFLICT (id) only absorbs the primary key, so the loser got
// a unique violation and a 500. With DO NOTHING an existing user returns no row, which
// Ent reports as sql.ErrNoRows.
func (s *server) ensureUser(ctx context.Context, p Principal) error {
	if err := s.Store.Ent.User.Create().
		SetID(p.UserID).
		SetUsername(p.Username).
		OnConflict(sql.DoNothing()).
		Exec(ctx); err != nil && !errors.Is(err, stdsql.ErrNoRows) {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}

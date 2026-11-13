// Package schema defines the Ent schema: the source of truth for the database tables.
// Migrations are generated from it (cmd/migratediff); see ADR-0004.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User is a CodeArena user. The ID is the Cognito "sub" claim.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			MaxLen(64).
			Immutable().
			Comment("Cognito sub claim"),
		field.String("username").
			NotEmpty().
			MaxLen(64).
			Unique(),
		field.Enum("role").
			Values("user", "admin").
			Default("user"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("submissions", Submission.Type),
		edge.To("contests", ContestParticipant.Type),
	}
}

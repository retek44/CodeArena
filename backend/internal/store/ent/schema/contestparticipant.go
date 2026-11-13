package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ContestParticipant registers a user for a contest. Only participants can submit in
// it and appear on its leaderboard.
type ContestParticipant struct {
	ent.Schema
}

// Fields of the ContestParticipant.
func (ContestParticipant) Fields() []ent.Field {
	return []ent.Field{
		field.Int("contest_id").
			Immutable(),
		field.String("user_id").
			Immutable(),
		field.Time("registered_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the ContestParticipant.
func (ContestParticipant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("participants").
			Field("contest_id").
			Unique().
			Required().
			Immutable(),
		edge.From("user", User.Type).
			Ref("contests").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

// Indexes of the ContestParticipant.
func (ContestParticipant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id").Unique(),
	}
}

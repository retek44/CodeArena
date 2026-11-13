package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Contest is a timed competition with ICPC rules (ADR-0016).
//
// The phase comes from the clock: before starts_at scheduled, then running, from
// freeze_at frozen (others see the leaderboard as it was at freeze_at), from ends_at
// ended. Status records what the lifecycle steps (Step Functions) have done; "final"
// means the results were announced.
type Contest struct {
	ent.Schema
}

// Fields of the Contest.
func (Contest) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").
			Unique().
			NotEmpty().
			Immutable(),
		field.String("title").
			NotEmpty(),
		field.Text("description_md").
			Default(""),
		field.Time("starts_at"),
		field.Time("freeze_at"),
		field.Time("ends_at"),
		field.Enum("status").
			Values("scheduled", "running", "frozen", "ended", "final").
			Default("scheduled"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the Contest.
func (Contest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("problems", ContestProblem.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("participants", ContestParticipant.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("submissions", Submission.Type),
	}
}

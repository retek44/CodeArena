package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ContestProblem puts a problem into a contest under a label (A, B, ...).
type ContestProblem struct {
	ent.Schema
}

// Fields of the ContestProblem.
func (ContestProblem) Fields() []ent.Field {
	return []ent.Field{
		field.Int("contest_id").
			Immutable(),
		field.Int("problem_id").
			Immutable(),
		field.String("label").
			NotEmpty().
			MaxLen(2),
	}
}

// Edges of the ContestProblem.
func (ContestProblem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("problems").
			Field("contest_id").
			Unique().
			Required().
			Immutable(),
		edge.From("problem", Problem.Type).
			Ref("contest_entries").
			Field("problem_id").
			Unique().
			Required().
			Immutable(),
	}
}

// Indexes of the ContestProblem.
func (ContestProblem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "label").Unique(),
		index.Fields("contest_id", "problem_id").Unique(),
	}
}

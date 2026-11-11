package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TestCase is one input with its expected output. Test cases are stored in the
// database for now; large cases move to S3 in phase 3 (ADR-0008).
type TestCase struct {
	ent.Schema
}

// Fields of the TestCase.
func (TestCase) Fields() []ent.Field {
	return []ent.Field{
		field.Int("problem_id"),
		field.Int("problem_version").
			Positive(),
		field.Int("ordinal").
			Positive().
			Comment("1-based position within the problem version"),
		field.Text("input"),
		field.Text("expected_output"),
		field.Bool("is_sample").
			Default(false).
			Comment("Sample cases are shown to users; all others stay hidden"),
	}
}

// Edges of the TestCase.
func (TestCase) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).
			Ref("test_cases").
			Field("problem_id").
			Unique().
			Required(),
	}
}

// Indexes of the TestCase.
func (TestCase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("problem_id", "problem_version", "ordinal").
			Unique(),
	}
}

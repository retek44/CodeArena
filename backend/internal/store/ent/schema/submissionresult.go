package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SubmissionResult is the verdict of one test case. Rows are bulk-inserted by
// the worker through sqlc (InsertSubmissionResults).
type SubmissionResult struct {
	ent.Schema
}

// Fields of the SubmissionResult.
func (SubmissionResult) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("submission_id", uuid.UUID{}),
		field.Int("ordinal").
			Positive(),
		field.Enum("verdict").
			Values("AC", "WA", "TLE", "MLE", "OLE", "RE", "SKIPPED"),
		field.Int("runtime_ms").
			NonNegative().
			Default(0),
		field.Int("memory_kb").
			NonNegative().
			Default(0),
	}
}

// Edges of the SubmissionResult.
func (SubmissionResult) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("submission", Submission.Type).
			Ref("results").
			Field("submission_id").
			Unique().
			Required(),
	}
}

// Indexes of the SubmissionResult.
func (SubmissionResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("submission_id", "ordinal").
			Unique(),
	}
}

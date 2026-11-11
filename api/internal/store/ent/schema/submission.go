package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Submission is code submitted for one problem version.
//
// Status flow: queued -> running -> done | failed. The queued -> running
// transition is a conditional update in sqlc (ClaimSubmission), so a message
// delivered twice is judged once.
type Submission struct {
	ent.Schema
}

// Fields of the Submission.
func (Submission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("user_id").
			Immutable(),
		field.Int("problem_id").
			Immutable(),
		field.Int("problem_version").
			Positive().
			Immutable(),
		field.Enum("language").
			Values("python").
			Immutable(),
		field.Text("source").
			NotEmpty().
			MaxLen(65536).
			Immutable(),
		field.String("idempotency_key").
			NotEmpty().
			MaxLen(128).
			Immutable(),
		field.String("request_hash").
			NotEmpty().
			MaxLen(64).
			Immutable().
			Comment("SHA-256 of the request body; detects key reuse with a different body"),
		field.Enum("status").
			Values("queued", "running", "done", "failed").
			Default("queued"),
		field.Enum("verdict").
			Values("AC", "WA", "TLE", "MLE", "OLE", "RE", "CE", "IE").
			Optional().
			Nillable(),
		field.Int("runtime_ms").
			NonNegative().
			Optional().
			Nillable(),
		field.Int("memory_kb").
			NonNegative().
			Optional().
			Nillable(),
		field.Text("compile_output").
			Optional().
			Nillable(),
		field.Int("attempt").
			NonNegative().
			Default(0),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("Hot-path sqlc queries set this explicitly"),
		field.Time("judged_at").
			Optional().
			Nillable(),
	}
}

// Edges of the Submission.
func (Submission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("submissions").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
		edge.From("problem", Problem.Type).
			Ref("submissions").
			Field("problem_id").
			Unique().
			Required().
			Immutable(),
		edge.To("results", SubmissionResult.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes of the Submission.
func (Submission) Indexes() []ent.Index {
	return []ent.Index{
		// Idempotent POST /api/submissions.
		index.Fields("user_id", "idempotency_key").
			Unique(),
		// GET /api/users/me/submissions, newest first.
		index.Fields("user_id", "created_at"),
		// Finding stuck submissions (running for too long).
		index.Fields("status", "updated_at"),
	}
}

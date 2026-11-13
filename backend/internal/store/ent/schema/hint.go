package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Hint is the AI hint for one judged submission (POST /api/submissions/{id}/hint).
// One hint per submission: asking again returns the stored text without a new model
// call. The rows also count against the per-user hint rate limit.
type Hint struct {
	ent.Schema
}

// Fields of the Hint.
func (Hint) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("submission_id", uuid.UUID{}).
			Immutable(),
		field.String("user_id").
			NotEmpty().
			Immutable(),
		field.Text("text"),
		// ok: model answer; blocked: the guardrail or the code filter replaced it.
		field.Enum("status").
			Values("ok", "blocked"),
		field.String("model").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the Hint.
func (Hint) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("submission", Submission.Type).
			Ref("hint").
			Field("submission_id").
			Unique().
			Required().
			Immutable(),
	}
}

// Indexes of the Hint.
func (Hint) Indexes() []ent.Index {
	return []ent.Index{
		// One hint per submission: the unique edge creates the unique index.
		// Rate limit: hints per user in a sliding window.
		index.Fields("user_id", "created_at"),
	}
}

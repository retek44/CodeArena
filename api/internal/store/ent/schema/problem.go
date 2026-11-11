package schema

import (
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Problem is a programming problem. Changing test cases bumps Version, so
// submissions always refer to the exact test set they were judged against.
type Problem struct {
	ent.Schema
}

// Fields of the Problem.
func (Problem) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").
			NotEmpty().
			MaxLen(64).
			Match(slugPattern).
			Unique(),
		field.String("title").
			NotEmpty().
			MaxLen(200),
		field.Enum("difficulty").
			Values("easy", "medium", "hard"),
		field.Text("statement_md"),
		field.Int("time_limit_ms").
			Range(100, 10000),
		field.Int("memory_limit_mb").
			Range(16, 512),
		field.Enum("checker").
			Values("exact").
			Default("exact"),
		field.Int("version").
			Positive().
			Default(1),
		field.Bool("published").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Problem.
func (Problem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("test_cases", TestCase.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("tags", Tag.Type),
		edge.To("submissions", Submission.Type),
	}
}

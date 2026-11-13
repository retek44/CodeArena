package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OutboxMessage is a message waiting to be published (transactional outbox).
// It is written in the same transaction as the change it announces, so a message
// is never lost and never sent for a change that was rolled back. The relay
// publishes pending rows and marks them sent (at-least-once delivery).
type OutboxMessage struct {
	ent.Schema
}

// Fields of the OutboxMessage.
func (OutboxMessage) Fields() []ent.Field {
	return []ent.Field{
		field.String("topic").
			NotEmpty().
			MaxLen(64).
			Immutable(),
		// Bytes, not field.JSON(json.RawMessage): the generated code then needs no
		// encoding/json/jsontext, which Go 1.26 only has behind GOEXPERIMENT=jsonv2.
		field.Bytes("payload").
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("available_at").
			Default(time.Now).
			Comment("Not published before this time (backoff after a failed publish)"),
		field.Time("sent_at").
			Optional().
			Nillable(),
		field.Int("attempts").
			NonNegative().
			Default(0),
		field.Text("last_error").
			Optional().
			Nillable(),
	}
}

// Indexes of the OutboxMessage.
func (OutboxMessage) Indexes() []ent.Index {
	return []ent.Index{
		// The relay only scans pending rows; sent rows do not grow the index.
		index.Fields("available_at", "id").
			Annotations(entsql.IndexWhere("sent_at IS NULL")),
	}
}

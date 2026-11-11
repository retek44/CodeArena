// Package ent holds the Ent schema and the generated client (ADR-0004).
// The generator is a Go tool dependency (go get -tool entgo.io/ent/cmd/ent), so it works
// in workspace mode and its version is pinned in go.mod. Regenerate after a schema change:
//
//	go generate ./internal/store/ent
package ent

//go:generate go tool ent generate --feature sql/versioned-migration,sql/upsert,sql/modifier ./schema

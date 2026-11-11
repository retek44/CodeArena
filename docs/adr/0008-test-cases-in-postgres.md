# ADR-0008: Test cases in PostgreSQL first, S3 later

- Status: accepted
- Date: 2026-11-11

## Context

The plan stores test case files in S3. In phase 2 there is no worker in AWS yet,
problems are small (the golden problems have inputs of a few bytes), and keeping
everything in one database makes local development and integration tests simple.

## Decision

- Phase 2 stores `input` and `expected_output` as `text` columns in `test_cases`,
  keyed by `(problem_id, problem_version, ordinal)`.
- A problem change that touches test cases bumps `problems.version`. Submissions store
  the `problem_version` they were judged against, so old verdicts stay explainable.
- One pgx connection pool serves both data access layers: sqlc uses it directly
  (`pgx/v5`), Ent gets a `*sql.DB` from `stdlib.OpenDBFromPool`.

## Consequences

- No S3 dependency in phase 2; Testcontainers needs only PostgreSQL.
- Large test sets would bloat the database and the runner request (6 MB Lambda limit).
  Phase 3 adds optional `s3_key_in` / `s3_key_out` columns (expand), the worker reads
  from S3 when they are set, and a later migration drops the text columns (contract).

-- Hot-path queries (ADR-0004). sqlc generates internal/store/db from this file.
-- Ent's UpdateDefault does not run here, so every UPDATE sets updated_at itself.

-- name: ClaimSubmission :one
-- Moves a queued submission to running. SQS delivers at least once, so the same
-- message can arrive twice; only the first claim returns a row.
UPDATE submissions
SET status = 'running',
    attempt = attempt + 1,
    updated_at = now()
WHERE id = $1
  AND status = 'queued'
RETURNING id, user_id, problem_id, problem_version, language, source, attempt;

-- name: CompleteSubmission :execrows
UPDATE submissions
SET status = 'done',
    verdict = sqlc.arg(verdict)::text,
    runtime_ms = sqlc.narg(runtime_ms)::bigint,
    memory_kb = sqlc.narg(memory_kb)::bigint,
    compile_output = sqlc.narg(compile_output)::text,
    judged_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'running';

-- name: RequeueSubmission :execrows
-- Returns a running submission to the queue after a worker or runner failure.
UPDATE submissions
SET status = 'queued',
    updated_at = now()
WHERE id = $1
  AND status = 'running';

-- name: FailSubmission :execrows
-- Gives up after the retry budget: the user sees IE (internal error).
UPDATE submissions
SET status = 'failed',
    verdict = 'IE',
    judged_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'running';

-- name: InsertSubmissionResults :exec
-- One statement for all test cases; a retried worker inserts nothing twice.
INSERT INTO submission_results (submission_id, ordinal, verdict, runtime_ms, memory_kb)
-- Several unnest() calls in one SELECT list are zipped element by element.
SELECT sqlc.arg(submission_id)::uuid,
       unnest(sqlc.arg(ordinals)::bigint[]),
       unnest(sqlc.arg(verdicts)::text[]),
       unnest(sqlc.arg(runtimes_ms)::bigint[]),
       unnest(sqlc.arg(memories_kb)::bigint[])
ON CONFLICT (submission_id, ordinal) DO NOTHING;

-- name: ListStuckSubmissions :many
-- Running submissions whose worker stopped reporting (input for a requeue job).
SELECT id, attempt
FROM submissions
WHERE status = 'running'
  AND updated_at < now() - sqlc.arg(older_than)::interval
ORDER BY updated_at
LIMIT sqlc.arg(max_rows)::int;

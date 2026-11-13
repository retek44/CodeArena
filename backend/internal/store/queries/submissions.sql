-- Hot-path queries (ADR-0004). sqlc generates internal/store/db from this file.
-- Ent's UpdateDefault does not run here, so every UPDATE sets updated_at itself.

-- name: ClaimSubmission :one
-- Moves a submission to running and takes a lease on it. SQS delivers at least once,
-- so the same message can arrive twice; only one claim returns a row. A running
-- submission whose lease expired (its worker crashed) can be claimed again.
UPDATE submissions
SET status = 'running',
    attempt = attempt + 1,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND (status = 'queued'
       OR (status = 'running' AND updated_at < now() - sqlc.arg(lease)::interval))
RETURNING id, user_id, problem_id, problem_version, language, source, attempt, contest_id;

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

-- name: FailExhaustedSubmissions :many
-- Sweeper: a running submission whose worker went silent after its last allowed
-- attempt gets IE instead of another try (a poison submission must not loop).
UPDATE submissions
SET status = 'failed',
    verdict = 'IE',
    judged_at = now(),
    updated_at = now()
WHERE id IN (
    SELECT s.id
    FROM submissions s
    WHERE s.status = 'running'
      AND s.updated_at < now() - sqlc.arg(running_after)::interval
      AND s.attempt >= sqlc.arg(max_attempts)::bigint
    ORDER BY s.updated_at
    LIMIT sqlc.arg(max_rows)::int
    FOR UPDATE SKIP LOCKED
)
RETURNING id, attempt;

-- name: RequeueStuckSubmissions :many
-- Sweeper: submissions whose queue message is gone (moved to the DLQ, or lost)
-- would wait forever. Running ones whose worker went silent and queued ones that
-- nobody claimed go back to queued, and a new outbox message announces each one in
-- the same statement. A duplicate message is harmless: the claim is idempotent.
-- SKIP LOCKED lets several sweepers (one per API task) run at once.
WITH stuck AS (
    SELECT s.id
    FROM submissions s
    WHERE (s.status = 'running' AND s.updated_at < now() - sqlc.arg(running_after)::interval
           AND s.attempt < sqlc.arg(max_attempts)::bigint)
       OR (s.status = 'queued' AND s.updated_at < now() - sqlc.arg(queued_after)::interval)
    ORDER BY s.updated_at
    LIMIT sqlc.arg(max_rows)::int
    FOR UPDATE SKIP LOCKED
), requeued AS (
    UPDATE submissions s
    SET status = 'queued',
        updated_at = now()
    FROM stuck
    WHERE s.id = stuck.id
    RETURNING s.id, s.attempt
), announced AS (
    -- Same payload as outbox.SubmissionCreated (version 1).
    INSERT INTO outbox_messages (topic, payload, created_at, available_at)
    SELECT sqlc.arg(topic)::text, jsonb_build_object('v', 1, 'submission_id', r.id), now(), now()
    FROM requeued r
)
SELECT id, attempt FROM requeued;

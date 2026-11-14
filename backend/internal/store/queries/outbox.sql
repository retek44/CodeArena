-- Transactional outbox (ADR-0011). The relay runs these inside one transaction.

-- name: LockPendingOutbox :many
-- SKIP LOCKED lets several relays (one per API task) work in parallel without
-- publishing the same row twice.
SELECT id, topic, payload, attempts, trace_parent
FROM outbox_messages
WHERE sent_at IS NULL
  AND available_at <= now()
ORDER BY id
LIMIT sqlc.arg(max_rows)::int
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxSent :exec
UPDATE outbox_messages
SET sent_at = now()
WHERE id = ANY(sqlc.arg(ids)::bigint[]);

-- name: MarkOutboxFailed :exec
-- Backs off before the next attempt; the error is kept for debugging.
UPDATE outbox_messages
SET attempts = attempts + 1,
    last_error = sqlc.arg(last_error)::text,
    available_at = now() + sqlc.arg(backoff)::interval
WHERE id = sqlc.arg(id);

-- name: DeleteSentOutbox :execrows
-- Housekeeping: sent rows are only kept for a short audit window.
DELETE FROM outbox_messages
WHERE sent_at < now() - sqlc.arg(older_than)::interval;

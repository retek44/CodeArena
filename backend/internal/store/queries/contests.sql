-- Contest leaderboard (ADR-0016). The source of truth: Valkey only caches what this
-- query returns, and the API falls back to it when Valkey is down.

-- name: ContestStandings :many
-- One row per participant and contest problem, with ICPC data: when the problem was
-- first accepted and how many rejected submissions came before that. Compile errors
-- and internal errors are not penalized. Only submissions made before the cutoff count
-- (freeze_at for the frozen leaderboard). user_id NULL means all participants.
WITH subs AS (
    SELECT s.user_id, s.problem_id, s.created_at, s.status, s.verdict
    FROM submissions s
    WHERE s.contest_id = sqlc.arg(contest_id)
      AND s.created_at < sqlc.arg(cutoff)::timestamptz
),
first_ac AS (
    SELECT user_id, problem_id, min(created_at) AS solved_at
    FROM subs
    WHERE status = 'done' AND verdict = 'AC'
    GROUP BY user_id, problem_id
)
SELECT p.user_id,
       u.username,
       cp.label,
       f.solved_at::timestamptz AS solved_at,
       count(sb.user_id) FILTER (
           WHERE sb.status = 'done'
             AND sb.verdict NOT IN ('AC', 'CE', 'IE')
             AND (f.solved_at IS NULL OR sb.created_at < f.solved_at)
       )::bigint AS rejected,
       count(sb.user_id) FILTER (WHERE sb.status IN ('queued', 'running'))::bigint AS pending
FROM contest_participants p
JOIN users u ON u.id = p.user_id
JOIN contest_problems cp ON cp.contest_id = p.contest_id
LEFT JOIN first_ac f ON f.user_id = p.user_id AND f.problem_id = cp.problem_id
LEFT JOIN subs sb ON sb.user_id = p.user_id AND sb.problem_id = cp.problem_id
WHERE p.contest_id = sqlc.arg(contest_id)
  AND (sqlc.narg(user_id)::text IS NULL OR p.user_id = sqlc.narg(user_id)::text)
GROUP BY p.user_id, u.username, cp.label, f.solved_at
ORDER BY p.user_id, cp.label;

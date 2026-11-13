import { Link, useParams } from "react-router";
import { useContest, useLeaderboard, useRegister } from "../api/hooks";
import { useAuth } from "../auth/AuthProvider";
import { ErrorMessage } from "../components/ErrorMessage";
import { phaseLabel } from "./ContestsPage";

export function ContestPage() {
  const { slug = "" } = useParams();
  const contest = useContest(slug);
  const register = useRegister(slug);
  const auth = useAuth();

  if (contest.isPending) return <p role="status">Loading contest…</p>;
  if (contest.isError) return <ErrorMessage error={contest.error} />;
  const c = contest.data;
  const open = c.phase !== "ended";

  return (
    <article>
      <h1>{c.title}</h1>
      <p className="meta">
        {phaseLabel(c.phase)} · {new Date(c.starts_at).toLocaleString()} – {new Date(c.ends_at).toLocaleTimeString()} ·
        leaderboard freezes at {new Date(c.freeze_at).toLocaleTimeString()} · {c.participants} registered
      </p>
      <div className="statement">{c.description_md}</div>

      {auth.user && c.registered === true && <p className="registered">You are registered.</p>}
      {auth.user && c.registered === false && open && (
        <p>
          <button type="button" onClick={() => register.mutate()} disabled={register.isPending}>
            Register
          </button>
        </p>
      )}
      {!auth.user && open && (
        <p>
          <Link to="/login">Sign in</Link> to take part.
        </p>
      )}
      {register.isError && <ErrorMessage error={register.error} />}

      <section aria-labelledby="problems-heading">
        <h2 id="problems-heading">Problems</h2>
        {c.problems.length === 0 ? (
          <p>The problems appear when the contest starts.</p>
        ) : (
          <ol className="contest-problems">
            {c.problems.map((p) => (
              <li key={p.label}>
                <strong>{p.label}</strong>{" "}
                {c.registered && (c.phase === "running" || c.phase === "frozen") ? (
                  <Link to={`/problems/${p.slug}?contest=${c.slug}&label=${p.label}`}>{p.title}</Link>
                ) : (
                  <Link to={`/problems/${p.slug}`}>{p.title}</Link>
                )}
              </li>
            ))}
          </ol>
        )}
      </section>

      <Leaderboard slug={slug} />
    </article>
  );
}

function Leaderboard({ slug }: { slug: string }) {
  const board = useLeaderboard(slug);

  if (board.isPending) return <p role="status">Loading leaderboard…</p>;
  if (board.isError) return <ErrorMessage error={board.error} />;
  const b = board.data;

  return (
    <section aria-labelledby="leaderboard-heading">
      <h2 id="leaderboard-heading">Leaderboard</h2>
      {b.frozen && (
        <p className="notice" role="note">
          The leaderboard is frozen: it shows the standings at the freeze. Final results appear when the contest ends.
        </p>
      )}
      {b.rows.length === 0 ? (
        <p>No participants yet.</p>
      ) : (
        <table className="leaderboard">
          <caption className="visually-hidden">Leaderboard, best first</caption>
          <thead>
            <tr>
              <th scope="col">#</th>
              <th scope="col">User</th>
              <th scope="col">Solved</th>
              <th scope="col">Penalty</th>
              {b.labels.map((l) => (
                <th scope="col" key={l}>
                  {l}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {b.rows.map((r) => (
              <tr key={r.username}>
                <td>{r.rank}</td>
                <td>{r.username}</td>
                <td>{r.solved}</td>
                <td>{r.penalty}</td>
                {r.problems.map((p) => (
                  <td key={p.label} className={p.solved_minute != null ? "solved" : p.rejected > 0 ? "rejected" : ""}>
                    {cellText(p)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

function cellText(p: { solved_minute: number | null; rejected: number; pending: number }): string {
  // ICPC notation: "+1 35'" = accepted at minute 35 after one rejection, "-2" = two rejections.
  if (p.solved_minute != null) return `+${p.rejected > 0 ? p.rejected : ""} ${p.solved_minute}'`;
  if (p.pending > 0) return "?";
  return p.rejected > 0 ? `-${p.rejected}` : "";
}

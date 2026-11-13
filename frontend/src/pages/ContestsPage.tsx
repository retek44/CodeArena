import { Link } from "react-router";
import { useContests } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";

export const PHASE_LABELS: Record<string, string> = {
  scheduled: "Upcoming",
  running: "Running",
  frozen: "Running (leaderboard frozen)",
  ended: "Ended",
};

export function phaseLabel(phase: string): string {
  return PHASE_LABELS[phase] ?? phase;
}

export function ContestsPage() {
  const contests = useContests();

  return (
    <section>
      <h1>Contests</h1>
      {contests.isPending && <p role="status">Loading contests…</p>}
      {contests.isError && <ErrorMessage error={contests.error} />}
      {contests.isSuccess && contests.data.length === 0 && <p>No contests yet.</p>}
      {contests.isSuccess && contests.data.length > 0 && (
        <table>
          <caption className="visually-hidden">Contests, newest first</caption>
          <thead>
            <tr>
              <th scope="col">Contest</th>
              <th scope="col">Starts</th>
              <th scope="col">Length</th>
              <th scope="col">Status</th>
            </tr>
          </thead>
          <tbody>
            {contests.data.map((c) => (
              <tr key={c.slug}>
                <td>
                  <Link to={`/contests/${c.slug}`}>{c.title}</Link>
                </td>
                <td>{new Date(c.starts_at).toLocaleString()}</td>
                <td>{Math.round((Date.parse(c.ends_at) - Date.parse(c.starts_at)) / 60_000)} min</td>
                <td>{phaseLabel(c.phase)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

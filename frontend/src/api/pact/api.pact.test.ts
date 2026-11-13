import { MatchersV3 as M } from "@pact-foundation/pact";
import { describe, expect, it } from "vitest";
import { ApiError } from "../client";
import {
  getContest,
  getLeaderboard,
  getProblem,
  getSubmission,
  listContests,
  listMySubmissions,
  listProblems,
  registerForContest,
  requestHint,
  submit,
} from "../requests";
import { SUBMISSION_ID, apiFor, authorized, newPact, problemDetails, timestamp } from "./provider";

// Consumer contract of the SPA: what it sends and the parts of the answers it reads.
// Provider states are set up by backend/internal/httpapi/pact_provider_test.go.

const PROBLEM = "problem a-plus-b exists";
const WRONG_ANSWER = "alice has a wrong-answer submission of a-plus-b";
const CONTEST = "contest weekly-1 is running";
const REGISTERED = "alice is registered for the running contest weekly-1";
const SOLVED = "alice solved problem A of the running contest weekly-1";

const submissionPath = M.fromProviderState("/api/submissions/${submissionId}", `/api/submissions/${SUBMISSION_ID}`);
const submissionId = M.fromProviderState("${submissionId}", SUBMISSION_ID);

const wrongAnswer = {
  id: submissionId,
  problem: "a-plus-b",
  language: M.string("python"),
  status: "done",
  verdict: "WA",
  runtime_ms: M.integer(12),
  memory_kb: M.integer(9000),
  created_at: timestamp(),
  cases: M.eachLike({ index: M.integer(1), verdict: M.string("WA") }),
};

const contestTimes = {
  starts_at: timestamp(),
  freeze_at: timestamp("2026-11-13T11:40:00Z"),
  ends_at: timestamp("2026-11-13T12:00:00Z"),
};

describe("problems", () => {
  it("lists published problems", async () => {
    await newPact()
      .addInteraction()
      .given(PROBLEM)
      .uponReceiving("a request for the first page of problems")
      .withRequest("GET", "/api/problems", (b) => b.query({ limit: "20" }))
      .willRespondWith(200, (b) =>
        b.jsonBody({
          items: M.eachLike({ slug: "a-plus-b", title: M.string("A + B"), difficulty: M.regex("^(easy|medium|hard)$", "easy") }),
          next_cursor: null,
        }),
      )
      .executeTest(async (server) => {
        const page = await listProblems(apiFor(server.url, false));
        expect(page.items[0]?.slug).toBe("a-plus-b");
        expect(page.next_cursor).toBeNull();
      });
  });

  it("reads a problem with its samples", async () => {
    await newPact()
      .addInteraction()
      .given(PROBLEM)
      .uponReceiving("a request for problem a-plus-b")
      .withRequest("GET", "/api/problems/a-plus-b")
      .willRespondWith(200, (b) =>
        b.jsonBody({
          slug: "a-plus-b",
          title: M.string("A + B"),
          difficulty: M.string("easy"),
          statement_md: M.string("Add two numbers."),
          time_limit_ms: M.integer(1000),
          memory_limit_mb: M.integer(64),
          samples: M.eachLike({ input: M.string("1 2\n"), output: M.string("3\n") }),
        }),
      )
      .executeTest(async (server) => {
        const problem = await getProblem(apiFor(server.url, false), "a-plus-b");
        expect(problem.samples).toHaveLength(1);
        expect(problem.time_limit_ms).toBe(1000);
      });
  });

  it("gets problem details for an unknown problem", async () => {
    await newPact()
      .addInteraction()
      .uponReceiving("a request for a problem that does not exist")
      .withRequest("GET", "/api/problems/no-such-problem")
      .willRespondWith(404, (b) => b.headers({ "Content-Type": "application/problem+json" }).jsonBody(problemDetails))
      .executeTest(async (server) => {
        const error = await getProblem(apiFor(server.url, false), "no-such-problem").catch((e: unknown) => e);
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).status).toBe(404);
      });
  });
});

describe("submissions", () => {
  it("submits a solution", async () => {
    await newPact()
      .addInteraction()
      .given(PROBLEM)
      .uponReceiving("a new submission")
      .withRequest("POST", "/api/submissions", (b) =>
        b
          .headers({ ...authorized, "Idempotency-Key": M.regex("^[A-Za-z0-9_-]{8,128}$", "click-0001") })
          .jsonBody({ problem: "a-plus-b", language: "python", source: "print(sum(map(int, input().split())))" }),
      )
      .willRespondWith(202, (b) =>
        b.jsonBody({
          id: M.uuid(SUBMISSION_ID),
          problem: "a-plus-b",
          language: "python",
          status: "queued",
          verdict: null,
          created_at: timestamp(),
          cases: [],
        }),
      )
      .executeTest(async (server) => {
        const created = await submit(apiFor(server.url), {
          problem: "a-plus-b",
          language: "python",
          source: "print(sum(map(int, input().split())))",
          idempotencyKey: "click-0001",
        });
        expect(created.status).toBe("queued");
        expect(created.id).toBe(SUBMISSION_ID);
      });
  });

  it("reads a judged submission", async () => {
    await newPact()
      .addInteraction()
      .given(WRONG_ANSWER)
      .uponReceiving("a request for alice's judged submission")
      .withRequest("GET", submissionPath, (b) => b.headers(authorized))
      .willRespondWith(200, (b) => b.jsonBody(wrongAnswer))
      .executeTest(async (server) => {
        const s = await getSubmission(apiFor(server.url), SUBMISSION_ID);
        expect(s.verdict).toBe("WA");
        expect(s.cases[0]?.verdict).toBe("WA");
      });
  });

  it("lists my submissions", async () => {
    await newPact()
      .addInteraction()
      .given(WRONG_ANSWER)
      .uponReceiving("a request for alice's submissions")
      .withRequest("GET", "/api/users/me/submissions", (b) => b.headers(authorized).query({ limit: "20" }))
      .willRespondWith(200, (b) => b.jsonBody({ items: M.eachLike(wrongAnswer), next_cursor: null }))
      .executeTest(async (server) => {
        const page = await listMySubmissions(apiFor(server.url));
        expect(page.items[0]?.problem).toBe("a-plus-b");
      });
  });

  it("asks for a hint", async () => {
    await newPact()
      .addInteraction()
      .given(WRONG_ANSWER)
      .uponReceiving("a hint request for alice's wrong answer")
      .withRequest("POST", M.fromProviderState("/api/submissions/${submissionId}/hint", `/api/submissions/${SUBMISSION_ID}/hint`), (b) =>
        b.headers(authorized),
      )
      .willRespondWith(200, (b) =>
        b.jsonBody({
          submission_id: submissionId,
          hint: M.string("Check the largest input."),
          status: M.string("ok"),
          created_at: timestamp(),
        }),
      )
      .executeTest(async (server) => {
        const hint = await requestHint(apiFor(server.url), SUBMISSION_ID);
        expect(hint.hint).not.toBe("");
      });
  });
});

describe("contests", () => {
  it("lists contests", async () => {
    await newPact()
      .addInteraction()
      .given(CONTEST)
      .uponReceiving("a request for the contests")
      .withRequest("GET", "/api/contests")
      .willRespondWith(200, (b) =>
        b.jsonBody({ items: M.eachLike({ slug: "weekly-1", title: M.string("Weekly 1"), ...contestTimes, phase: "running" }) }),
      )
      .executeTest(async (server) => {
        const contests = await listContests(apiFor(server.url, false));
        expect(contests[0]?.phase).toBe("running");
      });
  });

  it("reads a contest as a registered participant", async () => {
    await newPact()
      .addInteraction()
      .given(REGISTERED)
      .uponReceiving("a request for contest weekly-1 by a participant")
      .withRequest("GET", "/api/contests/weekly-1", (b) => b.headers(authorized))
      .willRespondWith(200, (b) =>
        b.jsonBody({
          slug: "weekly-1",
          title: M.string("Weekly 1"),
          ...contestTimes,
          phase: "running",
          description_md: M.string("Three problems."),
          problems: M.eachLike({ label: M.string("A"), slug: M.string("a-plus-b"), title: M.string("A + B") }),
          participants: M.integer(1),
          registered: true,
        }),
      )
      .executeTest(async (server) => {
        const contest = await getContest(apiFor(server.url), "weekly-1");
        expect(contest.registered).toBe(true);
        expect(contest.problems[0]?.label).toBe("A");
      });
  });

  it("registers for a contest", async () => {
    await newPact()
      .addInteraction()
      .given(CONTEST)
      .uponReceiving("a registration for contest weekly-1")
      .withRequest("POST", "/api/contests/weekly-1/register", (b) => b.headers(authorized))
      .willRespondWith(204)
      .executeTest(async (server) => {
        await expect(registerForContest(apiFor(server.url), "weekly-1")).resolves.toBeUndefined();
      });
  });

  it("submits to a contest", async () => {
    await newPact()
      .addInteraction()
      .given(REGISTERED)
      .uponReceiving("a contest submission")
      .withRequest("POST", "/api/submissions", (b) =>
        b
          .headers({ ...authorized, "Idempotency-Key": M.regex("^[A-Za-z0-9_-]{8,128}$", "click-0002") })
          .jsonBody({ problem: "a-plus-b", language: "python", source: "print(3)", contest: "weekly-1" }),
      )
      .willRespondWith(202, (b) =>
        b.jsonBody({
          id: M.uuid(SUBMISSION_ID),
          problem: "a-plus-b",
          language: "python",
          status: "queued",
          verdict: null,
          created_at: timestamp(),
          cases: [],
        }),
      )
      .executeTest(async (server) => {
        const created = await submit(apiFor(server.url), {
          problem: "a-plus-b",
          language: "python",
          source: "print(3)",
          contest: "weekly-1",
          idempotencyKey: "click-0002",
        });
        expect(created.status).toBe("queued");
      });
  });

  it("reads the leaderboard", async () => {
    await newPact()
      .addInteraction()
      .given(SOLVED)
      .uponReceiving("a request for the leaderboard of weekly-1")
      .withRequest("GET", "/api/contests/weekly-1/leaderboard")
      .willRespondWith(200, (b) =>
        b.jsonBody({
          contest: "weekly-1",
          phase: "running",
          frozen: false,
          labels: M.eachLike("A"),
          rows: M.eachLike({
            rank: M.integer(1),
            username: M.string("alice"),
            solved: M.integer(1),
            penalty: M.integer(5),
            problems: M.eachLike({
              label: M.string("A"),
              solved_minute: M.integer(5),
              rejected: M.integer(0),
              pending: M.integer(0),
            }),
          }),
        }),
      )
      .executeTest(async (server) => {
        const board = await getLeaderboard(apiFor(server.url, false), "weekly-1");
        expect(board.rows[0]?.solved).toBe(1);
        expect(board.frozen).toBe(false);
      });
  });
});

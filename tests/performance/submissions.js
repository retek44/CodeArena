// k6 load test: submissions through the whole judge chain.
//
// Every iteration submits a known-good solution, polls until the verdict is final and
// records the judge latency (POST -> done). Thresholds fail the run when the p95 judge
// latency is too high, when a submission is lost (no verdict within the timeout) or
// when a verdict is wrong.
//
// Environment:
//   BASE_URL        API origin (CloudFront URL in AWS, http://localhost:8080 locally)
//   TOKENS          comma-separated Cognito access tokens, one per virtual user (AWS)
//   DEV_USERS=1     use X-Dev-User headers instead (local stack, AUTH_MODE=dev)
//   VUS, DURATION   load shape (default 5 VUs for 1m)
//   PACING_S        minimum seconds between two submissions of one VU (default 6:
//                   the API allows 10 submissions per user and minute)
//   LATENCY_P95_MS  threshold for the p95 judge latency (default 10000)
//
//   docker run --rm -i --network host -e BASE_URL=http://localhost:8080 -e DEV_USERS=1 \
//     -v "$PWD/tests:/tests" grafana/k6:2.3.0 run /tests/performance/submissions.js

import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";

const BASE_URL = (__ENV.BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const TOKENS = (__ENV.TOKENS || "").split(",").filter(Boolean);
const DEV_USERS = __ENV.DEV_USERS === "1";
const PACING_S = Number(__ENV.PACING_S || 6);
const VERDICT_TIMEOUT_S = Number(__ENV.VERDICT_TIMEOUT_S || 120);

const SOURCE = `a, b = map(int, input().split())
print(a + b)
`;

export const options = {
  scenarios: {
    submissions: {
      executor: "constant-vus",
      vus: Number(__ENV.VUS || 5),
      duration: __ENV.DURATION || "1m",
      gracefulStop: `${VERDICT_TIMEOUT_S}s`,
    },
  },
  thresholds: {
    judge_latency_ms: [`p(95)<${Number(__ENV.LATENCY_P95_MS || 10000)}`],
    lost_submissions: ["count==0"],
    wrong_verdicts: ["count==0"],
    "http_req_failed{endpoint:submit}": ["rate<0.01"],
    checks: ["rate>0.99"],
  },
};

const judgeLatency = new Trend("judge_latency_ms", true);
const lost = new Counter("lost_submissions");
const wrong = new Counter("wrong_verdicts");
const accepted = new Rate("accepted");

export function setup() {
  if (!DEV_USERS && TOKENS.length === 0) {
    throw new Error("set TOKENS (Cognito access tokens) or DEV_USERS=1 (local stack)");
  }
  if (!DEV_USERS && TOKENS.length < Number(__ENV.VUS || 5)) {
    throw new Error(`need one token per VU: ${TOKENS.length} tokens for ${__ENV.VUS || 5} VUs`);
  }
}

function headers() {
  const h = { "Content-Type": "application/json" };
  if (DEV_USERS) {
    h["X-Dev-User"] = `k6-${__VU}`;
  } else {
    h.Authorization = `Bearer ${TOKENS[(__VU - 1) % TOKENS.length]}`;
  }
  return h;
}

function uuid() {
  // RFC 4122 version 4, good enough for idempotency keys.
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === "x" ? r : (r & 0x3) | 0x8).toString(16);
  });
}

export default function () {
  const started = Date.now();
  const res = http.post(
    `${BASE_URL}/api/submissions`,
    JSON.stringify({ problem: "a-plus-b", language: "python", source: SOURCE }),
    { headers: { ...headers(), "Idempotency-Key": uuid() }, tags: { endpoint: "submit" } },
  );
  if (!check(res, { "submission accepted (202)": (r) => r.status === 202 })) {
    sleep(PACING_S);
    return;
  }
  const id = res.json("id");

  let submission = null;
  while (Date.now() - started < VERDICT_TIMEOUT_S * 1000) {
    sleep(1);
    const poll = http.get(`${BASE_URL}/api/submissions/${id}`, {
      headers: headers(),
      tags: { endpoint: "poll", name: "/api/submissions/{id}" },
    });
    if (poll.status !== 200) continue;
    submission = poll.json();
    if (submission.status === "done" || submission.status === "failed") break;
  }

  if (!submission || (submission.status !== "done" && submission.status !== "failed")) {
    lost.add(1);
    check(null, { "verdict within the timeout": () => false });
  } else {
    judgeLatency.add(Date.now() - started);
    const ok = submission.verdict === "AC";
    accepted.add(ok);
    if (!ok) wrong.add(1);
    check(submission, { "verdict is AC": () => ok });
  }

  const elapsed = (Date.now() - started) / 1000;
  if (elapsed < PACING_S) sleep(PACING_S - elapsed);
}

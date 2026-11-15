# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Four audiences, all confirmed:

- **Practising programmers** solve problems in the browser, submit code and watch the
  verdict of every test case arrive live. They iterate: read, code, submit, fix, resubmit.
- **Contestants** take part in timed contests with an ICPC-style leaderboard (solved
  problems first, then penalty minutes; the board freezes before the end).
- **Teachers and organisers** are admins (Cognito group `admins`). They create problems,
  write statements in Markdown, upload test cases and publish problems for their group.
- **Portfolio visitors** (recruiters, interviewers, engineers) look at the live site and
  the repository to judge how the system is built and tested.

## Product Purpose

CodeArena is an online judge on AWS: problems, a browser editor (Monaco), sandboxed
judging in Python, C++, Java, JavaScript and Go, live per-test-case results over
WebSocket, contests with a live leaderboard, and problem management for admins.

Success means a user trusts the verdict: it is correct, it arrives quickly, and when it
is not "Accepted" the user can see which test cases failed and why (compiler output,
runtime error, time, memory) without guessing.

## Positioning

A judge you can trust. Untrusted code runs in runner Lambdas without network access and
with CPU, memory, process, file and output limits; expected outputs never reach the
runner; every test case reports its own verdict, time and memory; a change to a
problem's limits, checker or test cases creates a new version, so earlier submissions
keep the test cases they were judged with. The engineering behind this (test layers,
progressive delivery with automatic rollback) is documented in the repository and is
part of the evidence.

## Operating Context

- Desktop browser with a code editor is the main working scene; a solving session is a
  loop of reading the statement and samples, editing, submitting and reading results.
- Contests are time-boxed: phases follow the clock (upcoming, running, frozen, ended);
  the leaderboard updates live.
- Admins prepare problems as Markdown statements and `NAME.in` / `NAME.out` test files.
- Sign-in is Cognito (hosted UI, PKCE); locally a dev sign-in by user name.
- Verdict vocabulary is fixed: `AC` Accepted, `WA` Wrong answer, `TLE` Time limit,
  `MLE` Memory limit, `OLE` Output limit, `RE` Runtime error, `CE` Compile error,
  `IE` Internal error (the submission is retried).

## Capabilities and Constraints

- Frontend: React SPA (Vite, TanStack Query, React Router, Monaco), types generated from
  `contracts/openapi.yaml`; the API contract is the source of truth for data shown.
- Submissions are rate limited per user and site-wide; the UI explains the wait
  (Retry-After) instead of failing silently.
- Problem statements render Markdown without raw HTML or images.
- Cost is bounded: no paid AI calls (ADR-0021), small AWS footprint; features that add
  per-request cost need an explicit decision.
- Problems are written for this project; never copy statements or test data from
  LeetCode, Codeforces or other judges.
- Undecided: a public sign-up policy for groups or classes beyond the `admins` group.

## Brand Commitments

- Name: CodeArena. No logo or other brand assets exist yet.
- Voice: plain, precise English; short sentences; exact technical terms (verdict codes,
  limits in ms and MB). No hype.

## Evidence on Hand

- Demo recording: `docs/media/demo.gif` (sign-in, a wrong answer with per-test-case
  results, the fix accepted, a contest leaderboard).
- Seed problems written for the project: twenty problems in `tests/golden/` (from `a-plus-b`
  to `typo-distance` and `window-peaks`), with golden solutions per language; seed contests in
  `tests/golden/contests.json`.
- Architecture and decisions: `README.md`, `docs/PLAN.md`, `docs/adr/`.
- There are no real users, testimonials, usage numbers or press. Do not invent them.

## Product Principles

1. The verdict is the product: correct, per test case, and explained.
2. Never hide state: queued, running, judged, failed, frozen, rate limited are always
   visible and named.
3. The fast loop wins: reading, coding, submitting and fixing should need no detours.
4. Fairness is visible: same limits for everyone, versioned problems, contest rules
   stated where they apply.
5. Show the engineering honestly: claims about the system point to evidence in the
   repository.

## Accessibility & Inclusion

WCAG 2.1 AA. The Playwright suite runs axe (WCAG 2.1 A/AA) on the main pages; serious
and critical findings fail the build. English UI only.

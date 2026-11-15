# ADR-0023: Problem management for admins

- Status: accepted
- Date: 2026-11-15

## Context

Problems came only from the seed (`tests/golden/<slug>/`). PLAN.md asks for an admin who
creates problems, uploads test cases and versions problems, and sketches test case uploads
through presigned S3 URLs.

Facts that shape the decision:

- Test cases already live in Postgres (`test_cases`, text columns, one row per case and
  problem version). The worker reads them from there; nothing reads test data from S3.
- Submissions store the problem version they were judged with. A judged submission must
  keep pointing at the test cases it was judged with.
- Sign-in is Cognito (ADR-0009); access tokens carry the user's groups in `cognito:groups`.
- The problems of this project are small: a few dozen test cases, at most a few MB.

## Decision

- **Who:** members of the Cognito group `admins` (Terraform creates the group, a human adds
  users with `aws cognito-idp admin-add-user-to-group`, see SETUP.md). The API checks the
  group on every admin request (403 otherwise); the SPA only hides what the API refuses
  anyway. With `AUTH_MODE=dev` the users `admin` and `admin-<name>` are admins.
- **API:** `GET /api/users/me` (with `admin`), and `GET|POST /api/admin/problems`,
  `GET|PUT /api/admin/problems/{slug}`. A save sends the whole problem with all test cases
  as JSON (at most 100 cases and 4 MB of test data, body at most 12 MB). The slug cannot
  change.
- **Versions:** a change of the time or memory limit, the checker or the test cases saves a
  new version with a new set of test case rows; the rows of earlier versions stay, so
  earlier submissions keep theirs. Title, statement, difficulty and publishing do not
  change the version. Saves use optimistic locking on the version (409 on a concurrent
  save).
- **Test data in Postgres, not S3:** no presigned uploads. One request validates and stores
  everything in one transaction, there is no second store to keep consistent and no IAM
  for browser uploads.
- **SPA:** *Admin* in the navigation for admins: a list of all problems (published or not)
  and an editor with a live Markdown preview (the problem page's renderer), the checker
  settings and a test case editor. Files named `NAME.in` and `NAME.out` (or `.ans`) are
  paired into test cases in the browser.

## Consequences

- Test data size is bounded by the request and database row sizes. Problems with tens of
  MB of test data would need S3 (presigned uploads, the worker reading from S3); that can
  be added behind the same API when needed.
- Every version keeps a full copy of its test cases; old versions are not deleted.
- There is no delete: unpublishing hides a problem and keeps its submissions valid.
- Admin access is a Cognito group membership, managed outside the app. Removing a user
  from the group takes effect when their access token expires (at most one hour).

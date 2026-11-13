# Setup (phase 0, manual steps)

Do these steps once. After them, CI can plan and (from phase 5) deploy without any
long-lived AWS keys.

## 1. AWS account

1. Create a **standalone** AWS account. Do not use an AWS "project" (preconfigured environment):
   those accounts sit in an AWS-managed organization whose service control policies deny S3,
   Lambda, ECS, EC2 and more. Check with `aws s3api list-buckets`: it must not say "explicit deny".
   Create an AWS account. New accounts start on the Free plan with credits for 6 months.
   After that the account must move to the paid plan or it is closed. Plan the timing
   around the period when you need the project for interviews.
2. Turn on MFA for the root user. Do not use the root user after this step.
3. Create an admin identity for local work. The Free plan does not allow IAM Identity Center,
   so use an IAM user:
   - IAM → Users → Create user, with console access and `AdministratorAccess` attached directly.
   - Assign MFA to the user.
   - Sign in from the CLI with short-lived credentials (no access keys):

   ```bash
   aws login --profile CodeArena
   ```

   If Terraform cannot read these credentials, export them into the shell (PowerShell):

   ```powershell
   aws configure export-credentials --profile CodeArena --format powershell | Invoke-Expression
   ```

   Last resort: an access key for the IAM user (`aws configure --profile CodeArena`).
   Delete the key when you do not need it; CI never uses it.

4. Use the profile in every shell where you run Terraform:

   ```bash
   export AWS_PROFILE=CodeArena
   ```

5. Enable Bedrock model access for the AI hint (once per account, free until used):
   AWS console → Amazon Bedrock (eu-central-1) → Model access → Anthropic **Claude Haiku 5.5**
   and **Claude Sonnet 5.5** (the nightly hint evaluation uses it as judge). Anthropic models
   ask for a short use-case form the first time. Check:

   ```bash
   aws bedrock get-inference-profile --inference-profile-identifier eu.anthropic.claude-haiku-5-5 --query status
   ```

   Without access everything works except hints: the endpoint answers 503.

6. Optional: a sender for contest result e-mails. AWS console → Amazon SES (eu-central-1) →
   Identities → Create identity → e-mail address (or a domain you own), then confirm the
   mail. While the account is in the SES sandbox, recipients must be verified identities too
   (request production access in SES to mail every participant). Set the address as the
   `CONTEST_EMAIL_FROM` repository variable (section 4). Without it contests work, only the
   e-mails are skipped.

   Note: each deployed environment runs an ElastiCache Serverless (Valkey) cache for the
   contest leaderboards. It has a minimum hourly charge while it exists, so destroy idle
   environments.

## 2. Tools

| Tool | Version | Windows install |
|---|---|---|
| Go | 1.26+ | `winget install GoLang.Go` |
| Terraform | 1.16.x | `winget install Hashicorp.Terraform` |
| tflint | 0.64+ | `winget install TerraformLinters.tflint` or Docker |
| AWS CLI | v2 | `winget install Amazon.AWSCLI` |
| pre-commit | latest | `pip install pre-commit` |
| golangci-lint | 2.14 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0` |
| make | any | `winget install ezwinports.make` (or use WSL) |
| Docker | Desktop | already installed |

Then install the git hooks:

```bash
pre-commit install --install-hooks -t pre-commit -t commit-msg
```

## 3. Bootstrap stack

```bash
cd infra/bootstrap
cp terraform.tfvars.example terraform.tfvars   # set github_owner, the two GitHub IDs and your email
terraform init
terraform apply
```

The budget emails do not need confirmation. Check that the budget shows up in
**Billing → Budgets**.

### Move the bootstrap state into S3

The first apply wrote local state. Move it into the new bucket:

1. Run `terraform output state_bucket`.
2. Uncomment the `backend "s3"` block in `infra/bootstrap/versions.tf`, set `bucket`.
3. Run `terraform init -migrate-state` and answer `yes`.
4. Delete the local `terraform.tfstate*` files after the migration succeeds.

### Shared stack (ECR repositories)

```bash
cd infra/shared
terraform init -backend-config=../backend.hcl
terraform apply
```

### Local backend config for the environment stacks

```bash
cd infra
terraform -chdir=bootstrap output -raw backend_hcl > backend.hcl
```

`backend.hcl` is gitignored. Use it with `terraform init -backend-config=../../backend.hcl`
in `infra/envs/<env>` (the Makefile does this for you).

## 4. GitHub repository

1. Create the repository `<owner>/CodeArena` (public, so Actions minutes are free and the
   project is visible). The name must match `github_owner` / `github_repo` exactly,
   because the OIDC trust policy compares the `sub` claim case-sensitively.
   GitHub also puts the immutable owner and repository IDs into the claim
   (`repo:owner@123/name@456:...`); `github_owner_id` and `github_repository_id` in
   `terraform.tfvars` must match `curl -s https://api.github.com/repos/OWNER/REPO`
   (`.owner.id`, `.id`). Without them every CI login fails with
   "Not authorized to perform sts:AssumeRoleWithWebIdentity".
2. The Go module path is `github.com/retek44/CodeArena/backend`; a fork renames it in `backend/go.mod` and the imports.
3. Push `main`.
4. **Settings → Secrets and variables → Actions → Variables:**
   - `AWS_ACCOUNT_ID` = `terraform output -raw account_id`
   - `AWS_REGION` = `eu-central-1`
   - `TF_STATE_BUCKET` = `terraform output -raw state_bucket`
   - optional `ALERT_EMAIL`: alarm notifications (DLQ) of every deployed environment. AWS sends
     a confirmation mail after each deploy that creates the topic; until it is confirmed,
     alarms only show in CloudWatch.
   - optional `CONTEST_EMAIL_FROM`: verified SES sender of the contest result e-mails (step 6
     of section 1).

   **Secrets** (optional):
   - `RELEASE_PLEASE_TOKEN`: fine-grained token for this repository with *Contents* and
     *Pull requests* read/write. release-please opens the release PR with it, so CI runs on
     that PR (PRs opened with `GITHUB_TOKEN` start no workflows).
   - `SONAR_TOKEN`: sign in to [sonarcloud.io](https://sonarcloud.io) with GitHub, import the
     repository (organization `retek44`, project key `retek44_CodeArena`, see
     `sonar-project.properties`), switch the analysis method to *GitHub Actions* (turn off
     automatic analysis), and create a token. Without the secret the SonarCloud job skips.
5. **Settings → Environments:** create `shared`, `preview`, `dev`, `staging`, `prod`.
   For `prod`, add yourself as required reviewer and allow only the `main` branch.
6. **Settings → General → Pull Requests:** allow squash merge only,
   default commit message = PR title.
7. **Settings → Rules → Rulesets** for `main`: require a pull request, require the status
   checks `CI OK` and `Conventional PR title`, block force pushes.
8. **Settings → Code security:** Dependabot alerts, Dependabot security updates,
   secret scanning, push protection.

## 5. Check

Open a PR (for example `docs: update progress`). Expected:

- All jobs pass, `CI OK` is green.
- `Terraform plan (shared|dev|staging|prod)` runs and writes the plan to the job summary.
  Environments that were never deployed show only additions.

Then tick the manual items in [PROGRESS.md](PROGRESS.md).

## 6. Deploy and destroy

Every running environment costs money by the hour (RDS, ALB, Fargate, public IPv4), so
nothing deploys on push.

- **One environment:** Actions → Deploy → Run workflow → `dev`. Builds and signs the images
  once per commit, migrates, seeds, rolls out, then runs the post-deploy Playwright suite.
  The URL is in the run summary.
- **Promotion:** Actions → Release (from `main`) → dev → staging → prod, same images; prod
  waits for the reviewer of the `prod` environment.
- **Destroy:** Actions → Destroy → environment. dev and staging are also destroyed every
  night at 21:00 UTC; prod never automatically.
- **Release notes:** every merge to `main` updates the release PR of release-please
  (version + `CHANGELOG.md` from the Conventional Commit titles). Merging it tags `vX.Y.Z` and
  publishes a GitHub Release with an SPDX SBOM.

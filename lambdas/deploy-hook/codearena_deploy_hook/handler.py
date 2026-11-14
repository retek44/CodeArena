"""Deployment hooks (ADR-0020): a new version proves itself before production traffic.

One function, two callers:

  ECS blue/green   Lifecycle hook at POST_TEST_TRAFFIC_SHIFT. The test listener rule
                   sends requests that carry the test header to the green tasks; the
                   hook calls the API through CloudFront with that header and answers
                   SUCCEEDED, FAILED (ECS rolls back) or IN_PROGRESS (ECS calls again,
                   for example while a new CloudFront distribution is still deploying).

  CodeDeploy       BeforeAllowTraffic hook of a runner Lambda canary. The new version
                   judges an accepted A + B solution before the alias sends it any
                   traffic; the result goes back with PutLifecycleEventHookExecutionStatus.

The site URL and the test header value are in one SecureString parameter
(SITE_PARAMETER), written by Terraform after the CDN exists.
"""

from __future__ import annotations

import json
import logging
import os
import urllib.error
import urllib.request
from typing import Any, Protocol

logger = logging.getLogger()
logger.setLevel(logging.INFO)

TEST_HEADER = "X-Codearena-Test-Traffic"
TEST_STAGE = "POST_TEST_TRAFFIC_SHIFT"
# ECS calls again after about 30 seconds: 20 attempts cover a new CloudFront distribution.
MAX_ATTEMPTS = 20
HTTP_TIMEOUT_S = 10
RETRYABLE_STATUS = frozenset({502, 503, 504})

# Accepted solutions of tests/golden/a-plus-b for each runner language.
A_PLUS_B = {
    "python": "a, b = map(int, input().split())\nprint(a + b)\n",
    "cpp": (
        "#include <iostream>\n\nint main() {\n    long long a, b;\n"
        "    std::cin >> a >> b;\n    std::cout << a + b << '\\n';\n}\n"
    ),
}


class SmokeTestError(Exception):
    """The new version answered wrongly."""


class NotReadyError(Exception):
    """The new version cannot be reached yet; try again later."""


class SSM(Protocol):
    def get_parameter(self, **kwargs: Any) -> dict[str, Any]: ...


class CodeDeploy(Protocol):
    def get_deployment(self, **kwargs: Any) -> dict[str, Any]: ...
    def put_lifecycle_event_hook_execution_status(self, **kwargs: Any) -> dict[str, Any]: ...


class Lambda(Protocol):
    def invoke(self, **kwargs: Any) -> dict[str, Any]: ...
    def list_versions_by_function(self, **kwargs: Any) -> dict[str, Any]: ...


_clients: dict[str, Any] = {}


def _client(name: str) -> Any:
    if name not in _clients:
        import boto3  # noqa: PLC0415 - provided by the Lambda runtime

        _clients[name] = boto3.client(name)
    return _clients[name]


def ssm() -> SSM:
    client: SSM = _client("ssm")
    return client


def codedeploy() -> CodeDeploy:
    client: CodeDeploy = _client("codedeploy")
    return client


def lambda_client() -> Lambda:
    client: Lambda = _client("lambda")
    return client


def handler(event: dict[str, Any], _context: object = None) -> dict[str, Any] | None:
    if "lifecycleStage" in event:
        return ecs_hook(event)
    if "DeploymentId" in event:
        codedeploy_hook(event)
        return None
    raise ValueError("neither an ECS lifecycle hook nor a CodeDeploy hook event")


# --- ECS blue/green --------------------------------------------------------------------


def ecs_hook(event: dict[str, Any]) -> dict[str, Any]:
    stage = event.get("lifecycleStage")
    if stage != TEST_STAGE:
        return {"hookStatus": "SUCCEEDED"}

    details = event.get("executionDetails") or {}
    target = details.get("targetServiceRevisionArn")
    weights = details.get("testTrafficWeights") or {}
    # A rollback calls the hook again with the weights reversed: nothing new to test.
    if target and weights.get(target) == 0:
        _log("rollback, test skipped", service=details.get("serviceArn"))
        return {"hookStatus": "SUCCEEDED"}

    attempt = _attempt(event) + 1
    try:
        site_url, test_header = site()
        checked = smoke_test_api(site_url, test_header)
    except NotReadyError as e:
        if attempt >= MAX_ATTEMPTS:
            _log("not reachable, giving up", attempt=attempt, error=str(e))
            return {"hookStatus": "FAILED"}
        _log("not reachable yet", attempt=attempt, error=str(e))
        return {"hookStatus": "IN_PROGRESS", "hookDetails": {"attempt": attempt}}
    except SmokeTestError as e:
        _log("smoke test failed", stage=stage, error=str(e), revision=target)
        return {"hookStatus": "FAILED"}
    _log("smoke test passed", stage=stage, checked=checked, revision=target)
    return {"hookStatus": "SUCCEEDED"}


def _attempt(event: dict[str, Any]) -> int:
    hook_details = event.get("hookDetails")
    if isinstance(hook_details, str):  # passed back as a JSON document
        try:
            hook_details = json.loads(hook_details)
        except ValueError:
            return 0
    if not isinstance(hook_details, dict):
        return 0
    try:
        return int(hook_details.get("attempt", 0))
    except (TypeError, ValueError):
        return 0


def site() -> tuple[str, str]:
    name = os.environ["SITE_PARAMETER"]
    try:
        value = ssm().get_parameter(Name=name, WithDecryption=True)["Parameter"]["Value"]
    except Exception as e:
        if type(e).__name__ == "ParameterNotFound":
            raise NotReadyError(f"parameter {name} does not exist yet") from e
        raise
    config = json.loads(value)
    site_url = str(config["site_url"]).rstrip("/")
    if not site_url.startswith("https://"):
        raise SmokeTestError(f"site URL must use HTTPS: {site_url}")
    return site_url, str(config["test_header"])


def fetch(url: str, headers: dict[str, str]) -> tuple[int, bytes]:
    """GET url; the status of an HTTP error is returned, not raised."""
    request = urllib.request.Request(url, headers=headers)  # noqa: S310 - HTTPS checked in site()
    try:
        with urllib.request.urlopen(request, timeout=HTTP_TIMEOUT_S) as response:  # noqa: S310
            return int(response.status), bytes(response.read())
    except urllib.error.HTTPError as e:
        return int(e.code), bytes(e.read())
    except (urllib.error.URLError, TimeoutError) as e:
        raise NotReadyError(f"GET {url}: {e}") from e


def _get_json(url: str, test_header: str) -> Any:
    status, body = fetch(url, {TEST_HEADER: test_header, "Accept": "application/json"})
    if status in RETRYABLE_STATUS:
        raise NotReadyError(f"GET {url}: status {status}")
    if status != 200:
        raise SmokeTestError(f"GET {url}: status {status}")
    try:
        return json.loads(body)
    except ValueError as e:
        raise SmokeTestError(f"GET {url}: body is not JSON") from e


def smoke_test_api(site_url: str, test_header: str) -> list[str]:
    """Problem list and one problem page through the test listener rule (database included)."""
    page = _get_json(f"{site_url}/api/problems", test_header)
    items = page.get("items") if isinstance(page, dict) else None
    if not isinstance(items, list) or not items:
        raise SmokeTestError("GET /api/problems: no problems (is the database seeded?)")
    slug = items[0].get("slug") if isinstance(items[0], dict) else None
    if not isinstance(slug, str) or not slug:
        raise SmokeTestError("GET /api/problems: first item has no slug")
    problem = _get_json(f"{site_url}/api/problems/{slug}", test_header)
    if not isinstance(problem, dict) or problem.get("slug") != slug:
        raise SmokeTestError(f"GET /api/problems/{slug}: wrong problem")
    return ["/api/problems", f"/api/problems/{slug}"]


# --- CodeDeploy (runner Lambda canary) -------------------------------------------------


def codedeploy_hook(event: dict[str, Any]) -> None:
    deployment_id = event["DeploymentId"]
    status = "Failed"
    try:
        info = codedeploy().get_deployment(deploymentId=deployment_id)["deploymentInfo"]
        # Each runner's deployment group is named after its function.
        function = info["deploymentGroupName"]
        version = latest_version(function)
        smoke_test_runner(function, version, deployment_id)
        status = "Succeeded"
        _log("runner smoke test passed", function=function, version=version)
    except Exception:
        logger.exception("runner smoke test failed (deployment %s)", deployment_id)
    finally:
        # Always answer: CodeDeploy waits an hour for a hook that stays silent.
        codedeploy().put_lifecycle_event_hook_execution_status(
            deploymentId=deployment_id,
            lifecycleEventHookExecutionId=event["LifecycleEventHookExecutionId"],
            status=status,
        )


def latest_version(function: str) -> str:
    """Highest published version: the one Terraform published for this deployment."""
    versions: list[int] = []
    kwargs: dict[str, Any] = {"FunctionName": function}
    while True:
        page = lambda_client().list_versions_by_function(**kwargs)
        versions += [int(v["Version"]) for v in page.get("Versions", []) if v["Version"].isdigit()]
        marker = page.get("NextMarker")
        if not marker:
            break
        kwargs["Marker"] = marker
    if not versions:
        raise SmokeTestError(f"{function} has no published version")
    return str(max(versions))


def runner_language(function: str) -> str:
    _, sep, language = function.rpartition("-runner-")
    if not sep or language not in A_PLUS_B:
        raise SmokeTestError(f"{function} is not a runner function")
    return language


def runner_request(language: str, deployment_id: str) -> dict[str, Any]:
    return {
        "protocol_version": 1,
        "submission_id": f"deploy-hook-{deployment_id}"[:64],
        "language": language,
        "source": A_PLUS_B[language],
        "limits": {"time_ms": 2000, "memory_mb": 256},
        "test_cases": [{"id": "1", "input": "1 2\n"}],
    }


def smoke_test_runner(function: str, version: str, deployment_id: str) -> None:
    language = runner_language(function)
    response = lambda_client().invoke(
        FunctionName=function,
        Qualifier=version,
        Payload=json.dumps(runner_request(language, deployment_id)).encode(),
    )
    payload = json.loads(response["Payload"].read())
    if response.get("FunctionError"):
        raise SmokeTestError(f"{function}:{version} failed: {payload}")
    if payload.get("compile", {}).get("status") != "ok":
        raise SmokeTestError(f"{function}:{version} compile status {payload.get('compile')}")
    results = payload.get("results") or []
    if len(results) != 1 or results[0].get("status") != "ok" or results[0].get("stdout", "").strip() != "3":
        raise SmokeTestError(f"{function}:{version} judged A + B wrongly: {results}")


def _log(message: str, **fields: Any) -> None:
    logger.info(json.dumps({"msg": message, **fields}, default=str))

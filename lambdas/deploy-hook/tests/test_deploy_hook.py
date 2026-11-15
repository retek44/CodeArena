from __future__ import annotations

import io
import json
from pathlib import Path
from typing import Any

import jsonschema
import pytest

from codearena_deploy_hook import handler

ROOT = Path(__file__).parents[3]
SITE = {"site_url": "https://d111.cloudfront.net", "test_header": "s3cret"}
REVISION = "arn:aws:ecs:eu-central-1:123456789012:service-revision/codearena-dev/codearena-dev-api/2"


class ParameterNotFound(Exception):  # same name as botocore's
    pass


class FakeSSM:
    def __init__(self, value: dict[str, str] | None) -> None:
        self.value = value

    def get_parameter(self, **kwargs: Any) -> dict[str, Any]:
        assert kwargs["WithDecryption"] is True
        if self.value is None:
            raise ParameterNotFound(kwargs["Name"])
        return {"Parameter": {"Value": json.dumps(self.value)}}


class FakeCodeDeploy:
    def __init__(self) -> None:
        self.statuses: list[dict[str, Any]] = []

    def get_deployment(self, **kwargs: Any) -> dict[str, Any]:
        return {
            "deploymentInfo": {
                "deploymentId": kwargs["deploymentId"],
                "deploymentGroupName": "codearena-prod-runner-cpp",
            }
        }

    def put_lifecycle_event_hook_execution_status(self, **kwargs: Any) -> dict[str, Any]:
        self.statuses.append(kwargs)
        return {}


class FakeLambda:
    def __init__(self, stdout: str = "3\n", function_error: bool = False) -> None:
        self.stdout = stdout
        self.function_error = function_error
        self.invocations: list[dict[str, Any]] = []

    def list_versions_by_function(self, **kwargs: Any) -> dict[str, Any]:
        if "Marker" not in kwargs:
            return {"Versions": [{"Version": "$LATEST"}, {"Version": "2"}, {"Version": "10"}], "NextMarker": "m"}
        return {"Versions": [{"Version": "9"}]}

    def invoke(self, **kwargs: Any) -> dict[str, Any]:
        self.invocations.append(kwargs)
        if self.function_error:
            body: dict[str, Any] = {"errorType": "Runtime.ExitError", "errorMessage": "boom"}
            return {"FunctionError": "Unhandled", "Payload": io.BytesIO(json.dumps(body).encode())}
        request = json.loads(kwargs["Payload"])
        body = {
            "protocol_version": 1,
            "submission_id": request["submission_id"],
            "compile": {"status": "ok", "output": ""},
            "results": [
                {
                    "id": "1",
                    "status": "ok",
                    "cpu_time_ms": 1,
                    "wall_time_ms": 2,
                    "memory_kb": 1024,
                    "exit_code": 0,
                    "signal": None,
                    "stdout": self.stdout,
                    "stderr": "",
                }
            ],
            "truncated": False,
        }
        return {"StatusCode": 200, "Payload": io.BytesIO(json.dumps(body).encode())}


@pytest.fixture(autouse=True)
def env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("SITE_PARAMETER", "/codearena-dev/deploy-hook")
    handler._clients.clear()


def use(monkeypatch: pytest.MonkeyPatch, **clients: Any) -> None:
    for name, client in clients.items():
        monkeypatch.setattr(handler, name, lambda c=client: c)


def serve(monkeypatch: pytest.MonkeyPatch, routes: dict[str, tuple[int, Any]]) -> list[tuple[str, dict[str, str]]]:
    calls: list[tuple[str, dict[str, str]]] = []

    def fetch(url: str, headers: dict[str, str]) -> tuple[int, bytes]:
        calls.append((url, headers))
        if url not in routes:
            return 404, b"{}"
        status, body = routes[url]
        if isinstance(body, Exception):
            raise body
        return status, json.dumps(body).encode()

    monkeypatch.setattr(handler, "fetch", fetch)
    return calls


HEALTHY = {
    "https://d111.cloudfront.net/api/problems": (200, {"items": [{"slug": "a-plus-b"}], "next_cursor": None}),
    "https://d111.cloudfront.net/api/problems/a-plus-b": (200, {"slug": "a-plus-b", "samples": []}),
}


def ecs_event(stage: str = "POST_TEST_TRAFFIC_SHIFT", test_weight: int = 100, **extra: Any) -> dict[str, Any]:
    return {
        "executionId": "f4fcae0f",
        "lifecycleStage": stage,
        "resourceArn": "arn:aws:ecs:eu-central-1:123456789012:service-deployment/codearena-dev/codearena-dev-api/x",
        "executionDetails": {
            "serviceArn": "arn:aws:ecs:eu-central-1:123456789012:service/codearena-dev/codearena-dev-api",
            "targetServiceRevisionArn": REVISION,
            "testTrafficWeights": {REVISION: test_weight, "blue": 100 - test_weight},
            "productionTrafficWeights": {REVISION: 0, "blue": 100},
        },
        **extra,
    }


def test_ecs_hook_passes_with_the_test_header(monkeypatch: pytest.MonkeyPatch) -> None:
    use(monkeypatch, ssm=FakeSSM(SITE))
    calls = serve(monkeypatch, HEALTHY)

    assert handler.handler(ecs_event()) == {"hookStatus": "SUCCEEDED"}
    assert [url for url, _ in calls] == list(HEALTHY)
    assert all(headers[handler.TEST_HEADER] == "s3cret" for _, headers in calls)


def test_ecs_hook_fails_on_a_server_error(monkeypatch: pytest.MonkeyPatch) -> None:
    use(monkeypatch, ssm=FakeSSM(SITE))
    serve(monkeypatch, {**HEALTHY, "https://d111.cloudfront.net/api/problems": (500, {"status": 500})})

    assert handler.handler(ecs_event()) == {"hookStatus": "FAILED"}


def test_ecs_hook_fails_without_seeded_problems(monkeypatch: pytest.MonkeyPatch) -> None:
    use(monkeypatch, ssm=FakeSSM(SITE))
    serve(monkeypatch, {"https://d111.cloudfront.net/api/problems": (200, {"items": [], "next_cursor": None})})

    assert handler.handler(ecs_event()) == {"hookStatus": "FAILED"}


@pytest.mark.parametrize(
    ("ssm_value", "routes"),
    [
        (None, HEALTHY),  # Terraform has not written the parameter yet
        (SITE, {"https://d111.cloudfront.net/api/problems": (503, {})}),  # CloudFront still deploying
        (SITE, {"https://d111.cloudfront.net/api/problems": (0, handler.NotReadyError("DNS"))}),
    ],
)
def test_ecs_hook_waits_until_reachable(
    monkeypatch: pytest.MonkeyPatch, ssm_value: dict[str, str] | None, routes: dict[str, tuple[int, Any]]
) -> None:
    use(monkeypatch, ssm=FakeSSM(ssm_value))
    serve(monkeypatch, routes)

    first = handler.handler(ecs_event())
    assert first == {"hookStatus": "IN_PROGRESS", "hookDetails": {"attempt": 1}}
    again = handler.handler(ecs_event(hookDetails=json.dumps(first["hookDetails"])))
    assert again == {"hookStatus": "IN_PROGRESS", "hookDetails": {"attempt": 2}}
    last = handler.handler(ecs_event(hookDetails={"attempt": handler.MAX_ATTEMPTS - 1}))
    assert last == {"hookStatus": "FAILED"}


def test_ecs_hook_rejects_a_plain_http_site(monkeypatch: pytest.MonkeyPatch) -> None:
    use(monkeypatch, ssm=FakeSSM({"site_url": "http://d111.cloudfront.net", "test_header": "x"}))
    calls = serve(monkeypatch, HEALTHY)

    assert handler.handler(ecs_event()) == {"hookStatus": "FAILED"}
    assert calls == []


@pytest.mark.parametrize("event", [ecs_event(stage="PRE_SCALE_UP"), ecs_event(test_weight=0)])
def test_ecs_hook_skips_other_stages_and_rollbacks(monkeypatch: pytest.MonkeyPatch, event: dict[str, Any]) -> None:
    calls = serve(monkeypatch, {})

    assert handler.handler(event) == {"hookStatus": "SUCCEEDED"}
    assert calls == []


def codedeploy_event() -> dict[str, str]:
    return {"DeploymentId": "d-ABC123", "LifecycleEventHookExecutionId": "exec-1"}


def test_runner_hook_judges_the_newest_version(monkeypatch: pytest.MonkeyPatch) -> None:
    deploy, lam = FakeCodeDeploy(), FakeLambda()
    use(monkeypatch, codedeploy=deploy, lambda_client=lam)

    assert handler.handler(codedeploy_event()) is None
    assert deploy.statuses == [
        {"deploymentId": "d-ABC123", "lifecycleEventHookExecutionId": "exec-1", "status": "Succeeded"}
    ]
    (call,) = lam.invocations
    assert call["FunctionName"] == "codearena-prod-runner-cpp"
    assert call["Qualifier"] == "10"
    request = json.loads(call["Payload"])
    assert request["language"] == "cpp"
    assert "std::cout" in request["source"]


@pytest.mark.parametrize("lam", [FakeLambda(stdout="4\n"), FakeLambda(function_error=True)])
def test_runner_hook_reports_a_wrong_runner(monkeypatch: pytest.MonkeyPatch, lam: FakeLambda) -> None:
    deploy = FakeCodeDeploy()
    use(monkeypatch, codedeploy=deploy, lambda_client=lam)

    handler.handler(codedeploy_event())
    assert [s["status"] for s in deploy.statuses] == ["Failed"]


def test_runner_requests_follow_the_protocol() -> None:
    schema = json.loads((ROOT / "runners" / "protocol" / "request.schema.json").read_text(encoding="utf-8"))
    for language in handler.A_PLUS_B:
        jsonschema.validate(handler.runner_request(language, "d-" + "X" * 80), schema)


@pytest.mark.parametrize("language", ["python", "cpp", "java", "javascript", "go"])
def test_runner_sources_are_the_golden_accepted_solutions(language: str) -> None:
    ext = {"python": "py", "cpp": "cpp", "java": "java", "javascript": "js", "go": "go"}[language]
    golden = (ROOT / "tests" / "golden" / "a-plus-b" / "solutions" / f"ac.{ext}").read_text(encoding="utf-8")
    assert handler.A_PLUS_B[language].strip() == golden.strip()


def test_unknown_event() -> None:
    with pytest.raises(ValueError, match="neither"):
        handler.handler({"foo": "bar"})

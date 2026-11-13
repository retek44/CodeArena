from __future__ import annotations

import json
import time
import uuid
from pathlib import Path
from typing import Any

import jsonschema
import pytest
import yaml

from codearena_ws import handler

ASYNCAPI = yaml.safe_load((Path(__file__).parents[3] / "contracts" / "asyncapi.yaml").read_text(encoding="utf-8"))


def schema(name: str) -> dict[str, Any]:
    """A component schema of contracts/asyncapi.yaml with its $refs inlined."""

    def inline(node: Any) -> Any:
        if isinstance(node, dict):
            ref = node.get("$ref")
            if isinstance(ref, str) and ref.startswith("#/components/schemas/"):
                return inline(ASYNCAPI["components"]["schemas"][ref.rsplit("/", 1)[1]])
            return {k: inline(v) for k, v in node.items()}
        if isinstance(node, list):
            return [inline(v) for v in node]
        return node

    return dict(inline(ASYNCAPI["components"]["schemas"][name]))


def validate(payload: Any, name: str) -> None:
    jsonschema.validate(payload, schema(name), format_checker=jsonschema.FormatChecker())


class GoneException(Exception):  # same name as botocore's
    pass


class FakeTable:
    def __init__(self) -> None:
        self.items: dict[tuple[str, str], dict[str, Any]] = {}

    def _key(self, key: dict[str, Any]) -> tuple[str, str]:
        return key["pk"]["S"], key["sk"]["S"]

    def put_item(self, **kw: Any) -> dict[str, Any]:
        self.items[self._key(kw["Item"])] = kw["Item"]
        return {}

    def get_item(self, **kw: Any) -> dict[str, Any]:
        item = self.items.get(self._key(kw["Key"]))
        return {"Item": item} if item else {}

    def delete_item(self, **kw: Any) -> dict[str, Any]:
        old = self.items.pop(self._key(kw["Key"]), None)
        return {"Attributes": old} if old and kw.get("ReturnValues") == "ALL_OLD" else {}

    def query(self, **kw: Any) -> dict[str, Any]:
        pk = kw["ExpressionAttributeValues"][":pk"]["S"]
        return {"Items": [v for (p, _), v in self.items.items() if p == pk]}


class FakeManagement:
    def __init__(self) -> None:
        self.sent: list[tuple[str, dict[str, Any]]] = []
        self.gone: set[str] = set()

    def post_to_connection(self, **kw: Any) -> dict[str, Any]:
        if kw["ConnectionId"] in self.gone:
            raise GoneException()
        self.sent.append((kw["ConnectionId"], json.loads(kw["Data"])))
        return {}


@pytest.fixture
def table(monkeypatch: pytest.MonkeyPatch) -> FakeTable:
    t = FakeTable()
    monkeypatch.setattr(handler, "_dynamodb", t)
    monkeypatch.setenv("TABLE_NAME", "codearena-dev-ws")
    monkeypatch.setenv("WS_ENDPOINT", "https://ws.example/live")
    return t


@pytest.fixture
def api(monkeypatch: pytest.MonkeyPatch) -> FakeManagement:
    m = FakeManagement()
    monkeypatch.setattr(handler, "_management", {"https://ws.example/live": m})
    return m


def ws_event(connection_id: str, body: str | None = None, ticket: str | None = None) -> dict[str, Any]:
    event: dict[str, Any] = {
        "requestContext": {"connectionId": connection_id, "domainName": "ws.example", "stage": "live"},
    }
    if body is not None:
        event["body"] = body
    if ticket is not None:
        event["queryStringParameters"] = {"ticket": ticket}
    return event


def add_ticket(table: FakeTable, ticket: str, user: str, expires_in: int = 60) -> None:
    table.put_item(
        Item={
            "pk": {"S": f"ticket#{ticket}"},
            "sk": {"S": "ticket"},
            "user_id": {"S": user},
            "expires_at": {"N": str(int(time.time()) + expires_in)},
        }
    )


def subscribe(submission_id: str) -> str:
    msg = {"action": "subscribe", "submission_id": submission_id}
    validate(msg, "Subscribe")
    return json.dumps(msg)


def judged(submission_id: str, user: str, status: str = "done", verdict: str | None = "AC") -> dict[str, Any]:
    detail: dict[str, Any] = {
        "v": 1,
        "submission_id": submission_id,
        "user_id": user,
        "status": status,
        "at": "2026-11-13T10:00:00Z",
    }
    if verdict:
        detail["verdict"] = verdict
    validate(detail, "SubmissionUpdated")
    return {"source": "codearena.judge", "detail-type": "Submission updated", "detail": detail}


def test_a_ticket_opens_exactly_one_connection(table: FakeTable) -> None:
    add_ticket(table, "t1", "alice")

    assert handler.connect(ws_event("c1", ticket="t1")) == {"statusCode": 200}
    assert table.items[("conn#c1", "conn")]["user_id"] == {"S": "alice"}
    assert handler.connect(ws_event("c2", ticket="t1")) == {"statusCode": 401}
    assert ("conn#c2", "conn") not in table.items


@pytest.mark.parametrize("ticket", [None, "", "unknown", "x" * 200])
def test_connect_without_a_valid_ticket_is_refused(table: FakeTable, ticket: str | None) -> None:
    assert handler.connect(ws_event("c1", ticket=ticket)) == {"statusCode": 401}


def test_expired_ticket_is_refused(table: FakeTable) -> None:
    add_ticket(table, "old", "alice", expires_in=-1)
    assert handler.connect(ws_event("c1", ticket="old")) == {"statusCode": 401}


def test_updates_reach_only_the_owner(table: FakeTable, api: FakeManagement) -> None:
    sid = str(uuid.uuid4())
    add_ticket(table, "ta", "alice")
    add_ticket(table, "tm", "mallory")
    handler.connect(ws_event("alice-tab", ticket="ta"))
    handler.connect(ws_event("mallory-tab", ticket="tm"))
    assert handler.message(ws_event("alice-tab", body=subscribe(sid))) == {"statusCode": 200}
    assert handler.message(ws_event("mallory-tab", body=subscribe(sid))) == {"statusCode": 200}

    result = handler.push(judged(sid, "alice", verdict="WA"))

    assert result == {"sent": 1, "gone": 0}
    assert api.sent == [
        ("alice-tab", {"type": "submission_update", "submission_id": sid, "status": "done", "verdict": "WA"})
    ]
    validate(api.sent[0][1], "SubmissionUpdate")
    assert "user_id" not in api.sent[0][1]


def test_running_update_has_no_verdict(table: FakeTable, api: FakeManagement) -> None:
    sid = str(uuid.uuid4())
    add_ticket(table, "t", "alice")
    handler.connect(ws_event("c", ticket="t"))
    handler.message(ws_event("c", body=subscribe(sid)))

    handler.push(judged(sid, "alice", status="running", verdict=None))

    validate(api.sent[0][1], "SubmissionUpdate")
    assert "verdict" not in api.sent[0][1]


def test_closed_connections_are_cleaned_up(table: FakeTable, api: FakeManagement) -> None:
    sid = str(uuid.uuid4())
    for conn in ("open", "closed"):
        add_ticket(table, f"t-{conn}", "alice")
        handler.connect(ws_event(conn, ticket=f"t-{conn}"))
        handler.message(ws_event(conn, body=subscribe(sid)))
    api.gone.add("closed")

    assert handler.push(judged(sid, "alice")) == {"sent": 1, "gone": 1}
    assert (f"sub#{sid}", "conn#closed") not in table.items
    assert (f"sub#{sid}", "conn#open") in table.items


@pytest.mark.parametrize(
    "body",
    [
        None,
        "not json",
        "[]",
        '{"action": "subscribe"}',
        '{"action": "unsubscribe", "submission_id": "6f1c1d64-0d8c-4f39-9c55-2a0d0b8f9e11"}',
        '{"action": "subscribe", "submission_id": "not-a-uuid"}',
        '{"action": "subscribe", "submission_id": "6f1c1d64-0d8c-4f39-9c55-2a0d0b8f9e11", "x": 1}',
        '{"action": "subscribe", "submission_id": "' + "a" * 2000 + '"}',
    ],
)
def test_invalid_messages_get_an_error(table: FakeTable, api: FakeManagement, body: str | None) -> None:
    add_ticket(table, "t", "alice")
    handler.connect(ws_event("c", ticket="t"))

    assert handler.message(ws_event("c", body=body)) == {"statusCode": 400}
    connection, error = api.sent[0]
    assert connection == "c"
    validate(error, "Error")
    assert not [k for k in table.items if k[0].startswith("sub#")]


def test_subscribe_needs_a_known_connection(table: FakeTable, api: FakeManagement) -> None:
    assert handler.message(ws_event("stranger", body=subscribe(str(uuid.uuid4())))) == {"statusCode": 403}


def test_disconnect_forgets_the_connection(table: FakeTable) -> None:
    add_ticket(table, "t", "alice")
    handler.connect(ws_event("c", ticket="t"))
    handler.disconnect(ws_event("c"))
    assert ("conn#c", "conn") not in table.items


def test_other_errors_are_raised(table: FakeTable, api: FakeManagement, monkeypatch: pytest.MonkeyPatch) -> None:
    def boom(**_: Any) -> dict[str, Any]:
        raise RuntimeError("throttled")

    monkeypatch.setattr(api, "post_to_connection", boom)
    with pytest.raises(RuntimeError):
        handler.send("https://ws.example/live", "c", {"type": "error", "message": "x"})


def test_route_dispatches_on_the_route_key(table: FakeTable, api: FakeManagement) -> None:
    add_ticket(table, "t", "alice")
    connect = ws_event("c", ticket="t")
    connect["requestContext"]["routeKey"] = "$connect"
    assert handler.route(connect) == {"statusCode": 200}

    sub = ws_event("c", body=subscribe(str(uuid.uuid4())))
    sub["requestContext"]["routeKey"] = "subscribe"
    assert handler.route(sub) == {"statusCode": 200}

    bye = ws_event("c")
    bye["requestContext"]["routeKey"] = "$disconnect"
    assert handler.route(bye) == {"statusCode": 200}
    assert ("conn#c", "conn") not in table.items


def contest_event(detail_type: str, contest: str, phase: str | None = None) -> dict[str, Any]:
    detail: dict[str, Any] = {"v": 1, "contest": contest, "at": "2026-11-13T10:00:00Z"}
    if phase:
        detail["phase"] = phase
    validate(detail, "ContestUpdated")
    return {"source": "codearena.judge", "detail-type": detail_type, "detail": detail}


def test_contest_updates_reach_every_watcher(table: FakeTable, api: FakeManagement) -> None:
    for user in ("alice", "bob"):
        add_ticket(table, f"t-{user}", user)
        handler.connect(ws_event(user, ticket=f"t-{user}"))
        msg = {"action": "subscribe", "contest": "weekly-1"}
        validate(msg, "Subscribe")
        assert handler.message(ws_event(user, body=json.dumps(msg))) == {"statusCode": 200}

    assert handler.push(contest_event("Leaderboard updated", "weekly-1")) == {"sent": 2, "gone": 0}
    assert {c for c, _ in api.sent} == {"alice", "bob"}
    for _, message in api.sent:
        validate(message, "LeaderboardUpdate")
        assert message == {"type": "leaderboard_update", "contest": "weekly-1"}

    api.sent.clear()
    handler.push(contest_event("Contest phase changed", "weekly-1", phase="frozen"))
    validate(api.sent[0][1], "ContestPhase")
    assert api.sent[0][1]["phase"] == "frozen"

    # Other contests' watchers get nothing.
    api.sent.clear()
    assert handler.push(contest_event("Leaderboard updated", "weekly-2")) == {"sent": 0, "gone": 0}


@pytest.mark.parametrize("contest", ["", "Weekly", "a" * 65, "weekly_1", 7])
def test_invalid_contest_subscriptions(table: FakeTable, api: FakeManagement, contest: Any) -> None:
    add_ticket(table, "t", "alice")
    handler.connect(ws_event("c", ticket="t"))
    body = json.dumps({"action": "subscribe", "contest": contest})
    assert handler.message(ws_event("c", body=body)) == {"statusCode": 400}

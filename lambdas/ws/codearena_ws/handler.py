"""WebSocket Lambdas (ADR-0015, contracts/asyncapi.yaml).

One package, four handlers:
  connect     $connect: consumes the single-use ticket from POST /api/ws/ticket and
              binds its user to the connection; no valid ticket, no connection.
  disconnect  $disconnect: forgets the connection.
  message     subscribe / $default: the browser subscribes to one submission.
  push        EventBridge rule: "Submission updated" goes to the subscribed connections
              of the submission's owner; "Leaderboard updated" and "Contest phase
              changed" go to every connection watching the contest.

DynamoDB table (one per environment), key pk + sk, TTL attribute expires_at:
  ticket#<ticket>        ticket        user_id                (1 minute)
  conn#<connection id>   conn          user_id                (2 hours, the API Gateway limit)
  sub#<submission id>    conn#<id>     user_id, connection_id (2 hours)
  contest#<slug>         conn#<id>     user_id, connection_id (2 hours)

Ownership is checked when pushing: an update goes only to connections of the user who
owns the submission, so subscribing to someone else's submission yields nothing.
"""

from __future__ import annotations

import json
import logging
import os
import re
import time
import uuid
from typing import Any, Protocol

logger = logging.getLogger()
logger.setLevel(logging.INFO)

CONNECTION_TTL_S = 2 * 60 * 60
MAX_MESSAGE_BYTES = 1_024


class DynamoDB(Protocol):
    def delete_item(self, **kwargs: Any) -> dict[str, Any]: ...
    def put_item(self, **kwargs: Any) -> dict[str, Any]: ...
    def get_item(self, **kwargs: Any) -> dict[str, Any]: ...
    def query(self, **kwargs: Any) -> dict[str, Any]: ...


class Management(Protocol):
    def post_to_connection(self, **kwargs: Any) -> dict[str, Any]: ...


class Gone(Exception):
    """The connection is closed (API Gateway GoneException)."""


def _s(value: str) -> dict[str, str]:
    return {"S": value}


def _n(value: int) -> dict[str, str]:
    return {"N": str(value)}


def _status(code: int) -> dict[str, int]:
    return {"statusCode": code}


# --- clients (created once per Lambda container; tests replace them) ------------------

_dynamodb: DynamoDB | None = None
_management: dict[str, Management] = {}


def dynamodb() -> DynamoDB:
    global _dynamodb  # noqa: PLW0603 - one client per Lambda container
    if _dynamodb is None:
        import boto3  # noqa: PLC0415 - provided by the Lambda runtime

        _dynamodb = boto3.client("dynamodb")
    return _dynamodb


def management(endpoint: str) -> Management:
    if endpoint not in _management:
        import boto3  # noqa: PLC0415

        _management[endpoint] = boto3.client("apigatewaymanagementapi", endpoint_url=endpoint)
    return _management[endpoint]


def send(endpoint: str, connection_id: str, message: dict[str, Any]) -> None:
    client = management(endpoint)
    try:
        client.post_to_connection(ConnectionId=connection_id, Data=json.dumps(message).encode())
    except Exception as err:
        # botocore raises client.exceptions.GoneException; compare by name so the
        # handler does not import botocore (tests use a stub).
        if type(err).__name__ in ("GoneException", "Gone"):
            raise Gone(connection_id) from err
        raise


def _table() -> str:
    return os.environ["TABLE_NAME"]


def _endpoint(event: dict[str, Any]) -> str:
    ctx = event["requestContext"]
    return f"https://{ctx['domainName']}/{ctx['stage']}"


# --- $connect ---------------------------------------------------------------------------


def connect(event: dict[str, Any], _context: object = None) -> dict[str, int]:
    ticket = (event.get("queryStringParameters") or {}).get("ticket", "")
    connection_id = event["requestContext"]["connectionId"]
    if not ticket or len(ticket) > 100:
        return _status(401)
    now = int(time.time())
    # Delete and read in one step: a ticket opens at most one connection.
    old = (
        dynamodb()
        .delete_item(
            TableName=_table(),
            Key={"pk": _s(f"ticket#{ticket}"), "sk": _s("ticket")},
            ReturnValues="ALL_OLD",
        )
        .get("Attributes")
    )
    if not old or int(old["expires_at"]["N"]) < now:
        logger.info(json.dumps({"msg": "connect refused", "reason": "ticket"}))
        return _status(401)
    user_id = old["user_id"]["S"]
    dynamodb().put_item(
        TableName=_table(),
        Item={
            "pk": _s(f"conn#{connection_id}"),
            "sk": _s("conn"),
            "user_id": _s(user_id),
            "expires_at": _n(now + CONNECTION_TTL_S),
        },
    )
    return _status(200)


# --- $disconnect ------------------------------------------------------------------------


def disconnect(event: dict[str, Any], _context: object = None) -> dict[str, int]:
    connection_id = event["requestContext"]["connectionId"]
    dynamodb().delete_item(TableName=_table(), Key={"pk": _s(f"conn#{connection_id}"), "sk": _s("conn")})
    # Subscriptions of the connection expire through TTL or are removed on the next push.
    return _status(200)


# --- subscribe / $default ---------------------------------------------------------------


_SLUG = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")


SUBSCRIBE_HELP = "expected {action: subscribe, submission_id: <uuid>} or {action: subscribe, contest: <slug>}"


def _decode(body: str | None) -> dict[str, Any] | None:
    if not body or len(body) > MAX_MESSAGE_BYTES:
        return None
    try:
        msg = json.loads(body)
    except json.JSONDecodeError:
        return None
    return msg if isinstance(msg, dict) and msg.get("action") == "subscribe" else None


def parse_subscribe(body: str | None) -> str | None:
    """Returns the subscription key of a valid subscribe message (asyncapi Subscribe):
    sub#<submission id> or contest#<slug>; None for anything else."""
    msg = _decode(body)
    if msg is None:
        return None
    if set(msg) == {"action", "submission_id"}:
        try:
            return f"sub#{uuid.UUID(str(msg['submission_id']))}"
        except ValueError:
            return None
    slug = msg.get("contest")
    if set(msg) == {"action", "contest"} and isinstance(slug, str) and len(slug) <= 64 and _SLUG.match(slug):
        return f"contest#{slug}"
    return None


def message(event: dict[str, Any], _context: object = None) -> dict[str, int]:
    connection_id = event["requestContext"]["connectionId"]
    endpoint = _endpoint(event)
    key = parse_subscribe(event.get("body"))
    if key is None:
        send(endpoint, connection_id, {"type": "error", "message": SUBSCRIBE_HELP})
        return _status(400)
    conn = dynamodb().get_item(TableName=_table(), Key={"pk": _s(f"conn#{connection_id}"), "sk": _s("conn")})
    item = conn.get("Item")
    if not item:
        return _status(403)
    dynamodb().put_item(
        TableName=_table(),
        Item={
            "pk": _s(key),
            "sk": _s(f"conn#{connection_id}"),
            "user_id": item["user_id"],
            "connection_id": _s(connection_id),
            "expires_at": _n(int(time.time()) + CONNECTION_TTL_S),
        },
    )
    return _status(200)


# --- EventBridge: Submission updated ----------------------------------------------------


def to_client(detail: dict[str, Any]) -> dict[str, Any]:
    """asyncapi SubmissionUpdated -> SubmissionUpdate. The user ID stays on the server."""
    out: dict[str, Any] = {
        "type": "submission_update",
        "submission_id": detail["submission_id"],
        "status": detail["status"],
    }
    if detail.get("verdict"):
        out["verdict"] = detail["verdict"]
    return out


def contest_message(detail_type: str, detail: dict[str, Any]) -> dict[str, Any]:
    """asyncapi ContestUpdated -> LeaderboardUpdate or ContestPhase."""
    if detail_type == "Contest phase changed":
        return {"type": "contest_phase", "contest": detail["contest"], "phase": detail["phase"]}
    return {"type": "leaderboard_update", "contest": detail["contest"]}


def _fan_out(endpoint: str, key: str, message_for: dict[str, Any], owner: str | None) -> tuple[int, int]:
    subs = dynamodb().query(
        TableName=_table(),
        KeyConditionExpression="pk = :pk",
        ExpressionAttributeValues={":pk": _s(key)},
    )
    sent = gone = 0
    for item in subs.get("Items", []):
        if owner is not None and item["user_id"]["S"] != owner:
            continue
        connection_id = item["connection_id"]["S"]
        try:
            send(endpoint, connection_id, message_for)
            sent += 1
        except Gone:
            gone += 1
            dynamodb().delete_item(TableName=_table(), Key={"pk": item["pk"], "sk": item["sk"]})
    return sent, gone


def push(event: dict[str, Any], _context: object = None) -> dict[str, int]:
    """EventBridge target. Submission updates go only to the owner's connections;
    contest updates (leaderboard, phase) go to everyone watching the contest."""
    detail_type = event.get("detail-type", "Submission updated")
    detail = event["detail"]
    endpoint = os.environ["WS_ENDPOINT"]
    if detail_type == "Submission updated":
        sent, gone = _fan_out(endpoint, f"sub#{detail['submission_id']}", to_client(detail), detail["user_id"])
    else:
        sent, gone = _fan_out(endpoint, f"contest#{detail['contest']}", contest_message(detail_type, detail), None)
    logger.info(json.dumps({"msg": "push", "detail_type": detail_type, "sent": sent, "gone": gone}))
    return {"sent": sent, "gone": gone}


# --- one function for the WebSocket routes ----------------------------------------------


def route(event: dict[str, Any], context: object = None) -> dict[str, int]:
    """Entry point of the WebSocket function: dispatches on the route key."""
    key = event["requestContext"].get("routeKey")
    if key == "$connect":
        return connect(event, context)
    if key == "$disconnect":
        return disconnect(event, context)
    return message(event, context)

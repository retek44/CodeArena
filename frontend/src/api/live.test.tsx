import { screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, mockFetch, problem, renderApp } from "../test/render";
import { submission } from "../test/fixtures";

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  readonly url: string;
  sent: string[] = [];
  closed = false;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.closed = true;
  }
}

afterEach(() => {
  FakeWebSocket.instances = [];
  vi.unstubAllGlobals();
});

describe("live updates", () => {
  it("subscribes with a ticket and refetches when the server announces a change", async () => {
    vi.stubGlobal("WebSocket", FakeWebSocket);
    const queued = submission({ status: "queued" });
    const done = submission({ status: "done", verdict: "WA", cases: [{ index: 1, verdict: "WA" }] });
    let calls = 0;
    let changed = false;
    const api = mockFetch({
      [`GET /api/submissions/${queued.id}`]: () => {
        calls++;
        return json(changed ? done : queued);
      },
      "POST /api/ws/ticket": () =>
        json({ ticket: "t".repeat(43), url: "wss://ws.example/live", expires_at: "2026-11-13T10:01:00Z" }),
    });
    renderApp(`/submissions/${queued.id}`, { fetch: api.fetch });

    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const [socket] = FakeWebSocket.instances;
    if (!socket) throw new Error("no socket");
    expect(socket.url).toBe(`wss://ws.example/live?ticket=${"t".repeat(43)}`);

    socket.onopen?.();
    expect(JSON.parse(socket.sent[0] ?? "null")).toEqual({ action: "subscribe", submission_id: queued.id });

    // Other submissions and junk are ignored.
    const before = calls;
    socket.onmessage?.({ data: "not json" });
    socket.onmessage?.({ data: JSON.stringify({ type: "submission_update", submission_id: "other", status: "done" }) });

    changed = true;
    socket.onmessage?.({
      data: JSON.stringify({ type: "submission_update", submission_id: queued.id, status: "done", verdict: "WA" }),
    });
    // Far sooner than the 15 s safety poll: the update came over the socket.
    await waitFor(() => expect(screen.getByText(/^Result:/)).toHaveTextContent("WA Wrong answer"), { timeout: 2000 });
    expect(calls).toBeLessThanOrEqual(before + 2);
    await waitFor(() => expect(socket.closed).toBe(true));
  });

  it("polls when there is no WebSocket API", async () => {
    vi.stubGlobal("WebSocket", FakeWebSocket);
    const queued = submission({ status: "queued" });
    let calls = 0;
    const api = mockFetch({
      [`GET /api/submissions/${queued.id}`]: () => {
        calls++;
        return json(calls > 1 ? submission({ status: "done", verdict: "AC" }) : queued);
      },
      "POST /api/ws/ticket": () => problem(503, "live updates are not enabled in this environment"),
    });
    renderApp(`/submissions/${queued.id}`, { fetch: api.fetch });

    await waitFor(() => expect(screen.getByText(/^Result:/)).toHaveTextContent("AC Accepted"), { timeout: 3000 });
    expect(FakeWebSocket.instances).toHaveLength(0);
  });
});

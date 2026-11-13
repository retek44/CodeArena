import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import type { Api } from "./client";

// Live updates over the WebSocket API (ADR-0015, contracts/asyncapi.yaml). The socket
// only says "this changed"; the page then reads the data over HTTP as before, so the
// REST contract stays the single source of the data. Without a WebSocket API (local
// stack: the ticket endpoint answers 503) or when the socket fails, pages keep polling.

type Subscription = { action: "subscribe"; submission_id: string } | { action: "subscribe"; contest: string };

function parse(data: string): Record<string, unknown> | null {
  try {
    const message: unknown = JSON.parse(data);
    return typeof message === "object" && message !== null ? (message as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

/**
 * Opens a socket with a fresh ticket, sends `subscription`, and invalidates
 * `queryKeys` whenever a message passes `matches` (and once after subscribing, to
 * catch a change between the first read and the subscription). Returns true while
 * the socket is open.
 */
function useLive(
  api: Api,
  subscription: Subscription,
  matches: (message: Record<string, unknown>) => boolean,
  queryKeys: readonly (readonly unknown[])[],
  enabled: boolean,
): boolean {
  const queryClient = useQueryClient();
  const [connected, setConnected] = useState(false);
  // Stable dependencies: the effect reconnects only when what we watch changes.
  const subscriptionJSON = JSON.stringify(subscription);
  const keysJSON = JSON.stringify(queryKeys);

  useEffect(() => {
    if (!enabled || typeof WebSocket === "undefined") return;
    let socket: WebSocket | undefined;
    let cancelled = false;
    const keys = JSON.parse(keysJSON) as unknown[][];
    const refresh = () => keys.forEach((queryKey) => void queryClient.invalidateQueries({ queryKey }));

    void (async () => {
      const { data } = await api.POST("/api/ws/ticket", {});
      if (!data || cancelled) return;
      socket = new WebSocket(`${data.url}?ticket=${encodeURIComponent(data.ticket)}`);
      socket.onopen = () => {
        socket?.send(subscriptionJSON);
        setConnected(true);
        refresh();
      };
      socket.onmessage = (event: MessageEvent<string>) => {
        const message = parse(event.data);
        if (message && matches(message)) refresh();
      };
      socket.onclose = () => setConnected(false);
      socket.onerror = () => setConnected(false);
    })().catch(() => setConnected(false));

    return () => {
      cancelled = true;
      socket?.close();
      setConnected(false);
    };
    // `matches` is derived from the subscription, which is in the dependencies.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api, subscriptionJSON, keysJSON, enabled, queryClient]);

  return connected;
}

/** Live status of one submission while `enabled`. */
export function useLiveSubmission(api: Api, id: string, enabled: boolean): boolean {
  return useLive(
    api,
    { action: "subscribe", submission_id: id },
    (m) => m.type === "submission_update" && m.submission_id === id,
    [["submission", id]],
    enabled,
  );
}

/** Live leaderboard and phase of one contest while `enabled`. */
export function useLiveContest(api: Api, slug: string, enabled: boolean): boolean {
  return useLive(
    api,
    { action: "subscribe", contest: slug },
    (m) => (m.type === "leaderboard_update" || m.type === "contest_phase") && m.contest === slug,
    [
      ["leaderboard", slug],
      ["contest", slug],
    ],
    enabled,
  );
}

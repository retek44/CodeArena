import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { vi } from "vitest";
import { createApi } from "../api/client";
import { ApiContext } from "../api/hooks";
import { routes } from "../App";
import { type Auth, AuthProvider } from "../auth/AuthProvider";

export type Handler = (req: Request) => Response | Promise<Response>;

/** A fetch stub that answers "METHOD /path" routes and records every request. */
export function mockFetch(handlers: Record<string, Handler>) {
  const requests: Request[] = [];
  const fetchFn = vi.fn(async (input: Request) => {
    requests.push(input.clone());
    const url = new URL(input.url);
    const handler = handlers[`${input.method} ${url.pathname}`];
    if (!handler) return problem(404, `no handler for ${input.method} ${url.pathname}`);
    return handler(input);
  });
  return { fetch: fetchFn as unknown as typeof fetch, requests };
}

export function json(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });
}

export function problem(status: number, detail: string, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify({ type: "about:blank", title: "Error", status, detail }), {
    status,
    headers: { "Content-Type": "application/problem+json", ...headers },
  });
}

export function devAuth(user: string | null): Auth {
  return {
    mode: "dev",
    user: user ? { name: user } : null,
    ready: true,
    signIn: vi.fn(async () => {}),
    completeSignIn: vi.fn(async () => {}),
    signOut: vi.fn(async () => {}),
    headers: async (): Promise<Record<string, string>> => (user ? { "X-Dev-User": user } : {}),
  };
}

/** Renders the real routes at `path` with a stubbed API. */
export function renderApp(path: string, opts: { fetch: typeof fetch; user?: string | null }) {
  const auth = devAuth(opts.user === undefined ? "alice" : opts.user);
  const api = createApi(auth.headers, opts.fetch);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [path] });

  render(
    <AuthProvider value={auth}>
      <QueryClientProvider client={queryClient}>
        <ApiContext.Provider value={api}>
          <RouterProvider router={router} />
        </ApiContext.Provider>
      </QueryClientProvider>
    </AuthProvider>,
  );
  return { router, auth };
}

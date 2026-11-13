import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { createBrowserRouter, RouterProvider, type RouteObject } from "react-router";
import { ApiError, createApi } from "./api/client";
import { ApiContext } from "./api/hooks";
import { useAuth } from "./auth/AuthProvider";
import { Layout } from "./components/Layout";
import { AuthCallbackPage, LoginPage, NotFoundPage } from "./pages/AuthPages";
import { MySubmissionsPage } from "./pages/MySubmissionsPage";
import { ProblemPage } from "./pages/ProblemPage";
import { ProblemsPage } from "./pages/ProblemsPage";
import { SubmissionPage } from "./pages/SubmissionPage";

export const routes: RouteObject[] = [
  {
    element: <Layout />,
    children: [
      { path: "/", element: <ProblemsPage /> },
      { path: "/problems/:slug", element: <ProblemPage /> },
      { path: "/submissions", element: <MySubmissionsPage /> },
      { path: "/submissions/:id", element: <SubmissionPage /> },
      { path: "/login", element: <LoginPage /> },
      { path: "/auth/callback", element: <AuthCallbackPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
];

export function newQueryClient() {
  return new QueryClient({
    defaultOptions: {
      // 4xx answers will not change on retry; only retry network and 5xx errors.
      queries: { retry: (count, error) => count < 2 && !(error instanceof ApiError && error.status < 500) },
    },
  });
}

export function App() {
  const auth = useAuth();
  const [queryClient] = useState(newQueryClient);
  const [router] = useState(() => createBrowserRouter(routes));
  const api = useMemo(() => createApi(auth.headers), [auth.headers]);

  if (!auth.ready) return <p role="status">Loading…</p>;
  return (
    <QueryClientProvider client={queryClient}>
      <ApiContext.Provider value={api}>
        <RouterProvider router={router} />
      </ApiContext.Provider>
    </QueryClientProvider>
  );
}

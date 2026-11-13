import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, useContext } from "react";
import { type Api, type Difficulty, type Submission, toApiError } from "./client";

export const ApiContext = createContext<Api | null>(null);

export function useApi(): Api {
  const api = useContext(ApiContext);
  if (!api) throw new Error("useApi must be used inside <ApiContext.Provider>");
  return api;
}

const PAGE_SIZE = 20;

export function useProblems(difficulty?: Difficulty) {
  const api = useApi();
  return useInfiniteQuery({
    queryKey: ["problems", difficulty ?? "all"],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data, error, response } = await api.GET("/api/problems", {
        params: { query: { limit: PAGE_SIZE, cursor: pageParam, difficulty } },
      });
      if (!data) throw toApiError(error, response);
      return data;
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function useProblem(slug: string) {
  const api = useApi();
  return useQuery({
    queryKey: ["problem", slug],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/problems/{slug}", { params: { path: { slug } } });
      if (!data) throw toApiError(error, response);
      return data;
    },
  });
}

const FINAL_STATUSES: ReadonlySet<Submission["status"]> = new Set(["done", "failed"]);

export function isFinal(s: Submission): boolean {
  return FINAL_STATUSES.has(s.status);
}

/** Polls the submission until it has a verdict. */
export function useSubmission(id: string, pollMs = 1000) {
  const api = useApi();
  return useQuery({
    queryKey: ["submission", id],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/submissions/{id}", { params: { path: { id } } });
      if (!data) throw toApiError(error, response);
      return data;
    },
    refetchInterval: (query) => (query.state.data && isFinal(query.state.data) ? false : pollMs),
  });
}

export function useMySubmissions() {
  const api = useApi();
  return useInfiniteQuery({
    queryKey: ["my-submissions"],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data, error, response } = await api.GET("/api/users/me/submissions", {
        params: { query: { limit: PAGE_SIZE, cursor: pageParam } },
      });
      if (!data) throw toApiError(error, response);
      return data;
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export interface SubmitInput {
  problem: string;
  source: string;
  /** Generated once per click: retries of the same click reuse it (no duplicate submissions). */
  idempotencyKey: string;
}

export function useSubmit() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ problem, source, idempotencyKey }: SubmitInput) => {
      const { data, error, response } = await api.POST("/api/submissions", {
        params: { header: { "Idempotency-Key": idempotencyKey } },
        body: { problem, language: "python", source },
      });
      if (!data) throw toApiError(error, response);
      return data;
    },
    onSuccess: (submission) => {
      queryClient.setQueryData(["submission", submission.id], submission);
      void queryClient.invalidateQueries({ queryKey: ["my-submissions"] });
    },
  });
}

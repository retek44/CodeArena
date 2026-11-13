import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, useContext } from "react";
import { type Api, type Difficulty, type Submission } from "./client";
import { useLiveContest, useLiveSubmission } from "./live";
import {
  getContest,
  getLeaderboard,
  getProblem,
  getSubmission,
  listContests,
  listMySubmissions,
  listProblems,
  registerForContest,
  requestHint,
  submit,
  type SubmitInput,
} from "./requests";

export const ApiContext = createContext<Api | null>(null);

export function useApi(): Api {
  const api = useContext(ApiContext);
  if (!api) throw new Error("useApi must be used inside <ApiContext.Provider>");
  return api;
}

export function useProblems(difficulty?: Difficulty) {
  const api = useApi();
  return useInfiniteQuery({
    queryKey: ["problems", difficulty ?? "all"],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => listProblems(api, pageParam, difficulty),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function useProblem(slug: string) {
  const api = useApi();
  return useQuery({
    queryKey: ["problem", slug],
    queryFn: () => getProblem(api, slug),
  });
}

const FINAL_STATUSES: ReadonlySet<Submission["status"]> = new Set(["done", "failed"]);

export function isFinal(s: Submission): boolean {
  return FINAL_STATUSES.has(s.status);
}

/** Interval of the safety-net poll while the WebSocket is open (a lost event only delays the page). */
export const LIVE_POLL_MS = 15_000;

/**
 * Reads the submission until it has a verdict. With live updates (WebSocket) it
 * refetches when the server announces a change and polls only slowly; without them it
 * polls every `pollMs`.
 */
export function useSubmission(id: string, pollMs = 1000) {
  const api = useApi();
  // The cache read is fresh on every render: useQuery re-renders when the data changes.
  const cached = useQueryClient().getQueryData<Submission>(["submission", id]);
  const live = useLiveSubmission(api, id, cached !== undefined && !isFinal(cached));
  return useQuery({
    queryKey: ["submission", id],
    queryFn: () => getSubmission(api, id),
    refetchInterval: (q) => (q.state.data && isFinal(q.state.data) ? false : live ? LIVE_POLL_MS : pollMs),
  });
}

export function useMySubmissions() {
  const api = useApi();
  return useInfiniteQuery({
    queryKey: ["my-submissions"],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => listMySubmissions(api, pageParam),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function useSubmit() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SubmitInput) => submit(api, input),
    onSuccess: (submission) => {
      queryClient.setQueryData(["submission", submission.id], submission);
      void queryClient.invalidateQueries({ queryKey: ["my-submissions"] });
    },
  });
}

/** Asks for the AI hint of a failed submission. The API stores one hint per submission. */
export function useHint(submissionId: string) {
  const api = useApi();
  return useMutation({
    mutationKey: ["hint", submissionId],
    mutationFn: () => requestHint(api, submissionId),
  });
}

export function useContests() {
  const api = useApi();
  return useQuery({
    queryKey: ["contests"],
    queryFn: () => listContests(api),
  });
}

export function useContest(slug: string) {
  const api = useApi();
  return useQuery({
    queryKey: ["contest", slug],
    queryFn: () => getContest(api, slug),
    // The phase changes with the clock: re-read now and then (the socket speeds it up).
    refetchInterval: (q) => (q.state.data?.phase === "ended" ? false : 30_000),
  });
}

export function useRegister(slug: string) {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => registerForContest(api, slug),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["contest", slug] });
      void queryClient.invalidateQueries({ queryKey: ["leaderboard", slug] });
    },
  });
}

/** Leaderboard; refetched on live updates, otherwise polled while the contest runs. */
export function useLeaderboard(slug: string, pollMs = 10_000) {
  const api = useApi();
  const cached = useQueryClient().getQueryData<{ phase: string }>(["leaderboard", slug]);
  const running = cached !== undefined && cached.phase !== "ended";
  const live = useLiveContest(api, slug, running);
  return useQuery({
    queryKey: ["leaderboard", slug],
    queryFn: () => getLeaderboard(api, slug),
    refetchInterval: (q) => (q.state.data?.phase === "ended" ? false : live ? LIVE_POLL_MS * 2 : pollMs),
  });
}

export type ReviewGrade = "again" | "hard" | "good" | "easy";

export interface ReviewState {
  vocab_item_id: string;
  status: string;
  ease_factor: number;
  interval_days: number;
  repetition_count: number;
  next_due_at: string;
}

export interface VocabItem {
  id: string;
  term: string;
  kind: "word" | "phrase";
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
  source_text: string;
  source_url: string;
  notes: string;
  created_at: string;
  updated_at: string;
  archived_at?: string;
}

export interface AutocompleteItem {
  term: string;
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
}

export interface AutocompleteResult extends AutocompleteItem {
  error: string;
}

export interface VocabWithState {
  item: VocabItem;
  state: ReviewState;
}

export interface ReviewLog {
  id: string;
  user_id: string;
  vocab_item_id: string;
  grade: ReviewGrade;
  reviewed_at: string;
}

export interface ReviewHistoryEntry {
  log: ReviewLog;
  item: VocabItem;
  state: ReviewState;
}

export interface ReviewStats {
  reviewed_today: number;
  reviewed_7_days: number;
  active_cards: number;
  due_now: number;
  archived_cards: number;
}

export interface PageParams {
  limit?: number;
  offset?: number;
  q?: string;
  status?: string;
}

export interface PageResponse<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

const API_URL = (import.meta.env.VITE_API_URL as string | undefined) ?? "http://localhost:8080";

function getToken() {
  return localStorage.getItem("session_token") ?? "";
}

export function setToken(token: string) {
  localStorage.setItem("session_token", token);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}),
      ...(init?.headers ?? {})
    }
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Request failed" }));
    throw new Error(error.error ?? "Request failed");
  }
  return response.json();
}

function withQuery(path: string, params?: PageParams) {
  if (!params) return path;
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "" || value === null) continue;
    search.set(key, String(value));
  }
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}

export async function requestMagicLink(email: string) {
  return request<{ token: string; verification_url: string; expires_at: string }>("/auth/magic-link", {
    method: "POST",
    body: JSON.stringify({ email, base_url: window.location.origin })
  });
}

export async function verifyMagicLink(token: string) {
  return request<{ session: { token: string } }>("/auth/verify", {
    method: "POST",
    body: JSON.stringify({ token })
  });
}

export async function listVocab(params?: PageParams) {
  return request<PageResponse<VocabWithState>>(withQuery("/vocab", params));
}

export async function createVocab(payload: Partial<VocabItem>) {
  return request<{ item: VocabItem; state: ReviewState }>("/vocab", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function autocompleteVocab(items: AutocompleteItem[]) {
  return request<{ items: AutocompleteResult[] }>("/vocab/autocomplete", {
    method: "POST",
    body: JSON.stringify({ items })
  });
}

export async function updateVocab(vocabID: string, payload: Partial<VocabItem>) {
  return request<{ item: VocabItem }>(`/vocab/${vocabID}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export async function deleteVocab(vocabID: string) {
  return request<{ item: VocabItem }>(`/vocab/${vocabID}`, {
    method: "DELETE"
  });
}

export async function listDue() {
  return request<{ items: VocabWithState[] }>("/reviews/due");
}

export async function listReviewHistory(params?: PageParams) {
  return request<PageResponse<ReviewHistoryEntry>>(withQuery("/reviews/history", params));
}

export async function getReviewStats() {
  return request<{ stats: ReviewStats }>("/reviews/stats");
}

export async function gradeReview(vocabID: string, grade: ReviewGrade) {
  return request<{ state: ReviewState }>(`/reviews/${vocabID}/grade`, {
    method: "POST",
    body: JSON.stringify({ grade })
  });
}

export async function listNotificationJobs() {
  return request<{ items: Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }> }>("/notifications/jobs");
}

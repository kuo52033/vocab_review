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
  source_text: string;
  source_url: string;
  notes: string;
  created_at: string;
  updated_at: string;
  archived_at?: string;
}

export interface VocabWithState {
  item: VocabItem;
  state: ReviewState;
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

export async function listVocab() {
  return request<{ items: VocabWithState[] }>("/vocab");
}

export async function createVocab(payload: Partial<VocabItem>) {
  return request<{ item: VocabItem; state: ReviewState }>("/vocab", {
    method: "POST",
    body: JSON.stringify(payload)
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

export async function gradeReview(vocabID: string, grade: ReviewGrade) {
  return request<{ state: ReviewState }>(`/reviews/${vocabID}/grade`, {
    method: "POST",
    body: JSON.stringify({ grade })
  });
}

export async function listNotificationJobs() {
  return request<{ items: Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }> }>("/notifications/jobs");
}

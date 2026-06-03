import { CSSProperties, FormEvent, MouseEvent, useEffect, useRef, useState } from "react";
import {
  autocompleteVocab,
  AutocompleteItem,
  AutocompleteResult,
  clearToken,
  createVocab,
  deleteVocab,
  getVocabAudioURL,
  getReviewStats,
  gradeReview,
  isUnauthorizedError,
  listDue,
  listNotificationJobs,
  listVocab,
  requestMagicLink,
  ReviewGrade,
  ReviewStats,
  setToken,
  updateVocab,
  verifyMagicLink,
  VocabItem,
  VocabWithState
} from "./api";
import cockatielWalk from "./assets/cockatiel_walk.gif";

type CardDraft = {
  term: string;
  meaning: string;
  chinese: string;
  example_sentence: string;
  notes: string;
};

type AuthState = {
  email: string;
  magicLink?: string;
  magicMessage?: string;
  token: string;
};

type ActiveSection = "review" | "add" | "library";
type AddMode = "single" | "bulk";
type CockatielDirection = "forward" | "reverse";
type ReviewTransitionPhase = "idle" | "exit" | "wipe";
type ReviewTransition = {
  key: number;
  phase: ReviewTransitionPhase;
  x: number;
  y: number;
};
type PreparedReviewSession = {
  deck: QuizCard[];
  dueItems: VocabWithState[];
  stats: ReviewStats;
};
type SessionSummary = {
  reviewed: number;
  correct: number;
  wrong: number;
  lastNextDue?: string;
};
type QuizOption = {
  id: string;
  text: string;
  isCorrect: boolean;
};
type QuizCard = {
  card: VocabWithState;
  options: QuizOption[];
};
type ParsedImportCard = {
  term: string;
  meaning: string;
  chinese: string;
  example_sentence: string;
  part_of_speech: string;
  error?: string;
};

const allowedPartsOfSpeech = new Set([
  "",
  "noun",
  "verb",
  "adjective",
  "adverb",
  "phrase",
  "idiom",
  "phrasal_verb",
  "preposition",
  "conjunction",
  "interjection",
  "determiner",
  "pronoun",
  "other"
]);

const emptyForm: CardDraft = {
  term: "",
  meaning: "",
  chinese: "",
  example_sentence: "",
  notes: ""
};

const emptyStats: ReviewStats = {
  reviewed_today: 0,
  reviewed_7_days: 0,
  active_cards: 0,
  due_now: 0,
  archived_cards: 0
};

const reviewSessionSize = 12;
const libraryPageSize = 10;

function draftFromItem(item: VocabItem): CardDraft {
  return {
    term: item.term,
    meaning: item.meaning,
    chinese: item.chinese,
    example_sentence: item.example_sentence,
    notes: item.notes
  };
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

function parseImportLine(line: string): ParsedImportCard {
  if (line.includes("|")) {
    const parts = line.split("|").map((part) => part.trim());
    if (parts.length >= 5) {
      const [term = "", meaning = "", chinese = "", example_sentence = "", part_of_speech = ""] = parts;
      return { term, meaning, chinese, example_sentence, part_of_speech };
    }
    const [term = "", meaning = "", example_sentence = "", part_of_speech = ""] = parts;
    return { term, meaning, chinese: "", example_sentence, part_of_speech };
  }
  const separators = [" - ", "\t", ": ", "："];
  for (const separator of separators) {
    const index = line.indexOf(separator);
    if (index === -1) continue;
    return {
      term: line.slice(0, index).trim(),
      meaning: line.slice(index + separator.length).trim(),
      chinese: "",
      example_sentence: "",
      part_of_speech: ""
    };
  }
  return { term: line.trim(), meaning: "", chinese: "", example_sentence: "", part_of_speech: "" };
}

function parseBulkImport(input: string) {
  return input
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map(parseImportLine)
    .filter((card) => card.term);
}

function pageCount(totalItems: number, pageSize: number) {
  return Math.max(1, Math.ceil(totalItems / pageSize));
}

function pageItems<T>(items: T[], page: number, pageSize: number) {
  const start = (page - 1) * pageSize;
  return items.slice(start, start + pageSize);
}

function shuffleItems<T>(items: T[]) {
  return [...items].sort(() => Math.random() - 0.5);
}

function answerText(item: VocabItem) {
  return item.meaning.trim();
}

function partOfSpeechLabel(item: VocabItem) {
  return item.part_of_speech ? item.part_of_speech.replace(/_/g, " ") : "Word";
}

function reviewAccuracy(summary: SessionSummary) {
  return Math.round((summary.correct / Math.max(1, summary.reviewed)) * 100);
}

function reviewResultMessage(summary: SessionSummary) {
  const accuracy = reviewAccuracy(summary);
  if (accuracy === 100) return "Perfect review.";
  if (accuracy >= 80) return "Great work.";
  if (accuracy >= 50) return "Nice progress.";
  return "Keep going.";
}

function buildQuizDeck(dueCards: VocabWithState[], candidates: VocabWithState[], limit: number): QuizCard[] {
  const cardsWithAnswers = dueCards.filter(({ item }) => answerText(item));
  const candidateAnswers = candidates
    .filter(({ item }) => answerText(item))
    .map(({ item }) => ({ id: item.id, text: answerText(item) }));

  return shuffleItems(cardsWithAnswers)
    .slice(0, limit)
    .map((card) => {
      const correctText = answerText(card.item);
      const distractors = shuffleItems(
        candidateAnswers.filter((candidate) => candidate.id !== card.item.id && candidate.text !== correctText)
      ).slice(0, 3);

      return {
        card,
        options: shuffleItems([
          { id: `${card.item.id}-correct`, text: correctText, isCorrect: true },
          ...distractors.map((distractor) => ({
            id: `${card.item.id}-${distractor.id}`,
            text: distractor.text,
            isCorrect: false
          }))
        ])
      };
    })
    .filter((quizCard) => quizCard.options.length >= 2);
}

type PaginationProps = {
  label: string;
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
};

function Pagination({ label, page, totalPages, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null;

  return (
    <nav className="pagination" aria-label={`${label} pagination`}>
      <button type="button" className="ghost-button compact-button" onClick={() => onPageChange(page - 1)} disabled={page <= 1}>
        Previous
      </button>
      <span>
        Page {page} of {totalPages}
      </span>
      <button type="button" className="ghost-button compact-button" onClick={() => onPageChange(page + 1)} disabled={page >= totalPages}>
        Next
      </button>
    </nav>
  );
}

function normalizePartOfSpeech(value: string) {
  const normalized = value.trim().toLowerCase().replace(/[\s-]+/g, "_");
  if (allowedPartsOfSpeech.has(normalized)) return normalized;
  return normalized ? "other" : "";
}

function mergeAutocompleteResults(cards: ParsedImportCard[], results: AutocompleteResult[]): ParsedImportCard[] {
  return cards.map((card, index) => {
    const result = results[index];
    if (!result) return card;
    const partOfSpeech = normalizePartOfSpeech(result.part_of_speech || "");
    return {
      ...card,
      meaning: card.meaning || result.meaning || "",
      chinese: card.chinese || result.chinese || "",
      example_sentence: card.example_sentence || result.example_sentence || "",
      part_of_speech: card.part_of_speech || partOfSpeech,
      error: result.error || undefined
    };
  });
}

function formatBulkImportCards(cards: ParsedImportCard[]) {
  return cards
    .map((card) => {
      const fields = [
        card.term,
        card.meaning,
        card.chinese,
        card.example_sentence,
        card.part_of_speech
      ].map((value) => value.trim());
      while (fields.length > 1 && fields[fields.length - 1] === "") {
        fields.pop();
      }
      return fields.join(" | ");
    })
    .join("\n");
}

type AudioPlayButtonProps = {
  item: VocabItem;
  isPlaying: boolean;
  onPlay: (item: VocabItem) => void;
};

function hasPlayableAudio(item: VocabItem) {
  return item.audio?.status === "ready" && Boolean(item.audio.url || item.audio.storage_key);
}

function AudioPlayButton({ item, isPlaying, onPlay }: AudioPlayButtonProps) {
  if (!hasPlayableAudio(item)) return null;

  return (
    <button
      type="button"
      className={`audio-play-button${isPlaying ? " is-playing" : ""}`}
      aria-label={`${isPlaying ? "Pause" : "Play"} pronunciation for ${item.term}`}
      onClick={() => onPlay(item)}
    >
      {isPlaying ? "Ⅱ" : "▶"}
    </button>
  );
}

export function App() {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const termInputRef = useRef<HTMLInputElement>(null);
  const importPreviewRef = useRef<HTMLDivElement>(null);
  const bulkTextRef = useRef("");
  const refreshRequestIDRef = useRef(0);
  const [auth, setAuth] = useState<AuthState>({ email: "", token: localStorage.getItem("session_token") ?? "" });
  const [vocab, setVocab] = useState<VocabWithState[]>([]);
  const [due, setDue] = useState<VocabWithState[]>([]);
  const [vocabTotal, setVocabTotal] = useState(0);
  const [stats, setStats] = useState<ReviewStats>(emptyStats);
  const [jobs, setJobs] = useState<Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }>>([]);
  const [form, setForm] = useState(emptyForm);
  const [bulkText, setBulkText] = useState("");
  const [enrichedCards, setEnrichedCards] = useState<ParsedImportCard[] | null>(null);
  const [isEnriching, setIsEnriching] = useState(false);
  const [enrichmentError, setEnrichmentError] = useState("");
  const [editingID, setEditingID] = useState("");
  const [editDraft, setEditDraft] = useState<CardDraft>(emptyForm);
  const [query, setQuery] = useState("");
  const [activeSection, setActiveSection] = useState<ActiveSection>("review");
  const [addMode, setAddMode] = useState<AddMode>("bulk");
  const [libraryPage, setLibraryPage] = useState(1);
  const [sessionDeck, setSessionDeck] = useState<QuizCard[]>([]);
  const [sessionIndex, setSessionIndex] = useState(0);
  const [sessionCorrectCount, setSessionCorrectCount] = useState(0);
  const [sessionWrongCount, setSessionWrongCount] = useState(0);
  const [sessionSummary, setSessionSummary] = useState<SessionSummary | null>(null);
  const [selectedOptionID, setSelectedOptionID] = useState("");
  const [pendingNextDue, setPendingNextDue] = useState("");
  const [lastCreatedTerm, setLastCreatedTerm] = useState("");
  const [lastImportCount, setLastImportCount] = useState(0);
  const [lastSkippedDuplicateTerm, setLastSkippedDuplicateTerm] = useState("");
  const [lastSkippedDuplicateCount, setLastSkippedDuplicateCount] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isStartingReview, setIsStartingReview] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isGrading, setIsGrading] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [isReviewIconLoaded, setIsReviewIconLoaded] = useState(false);
  const [playingAudioID, setPlayingAudioID] = useState("");
  const [cockatielRun, setCockatielRun] = useState(0);
  const [cockatielDirection, setCockatielDirection] = useState<CockatielDirection>(() => (
    Math.random() < 0.15 ? "reverse" : "forward"
  ));
  const [reviewTransition, setReviewTransition] = useState<ReviewTransition>({
    key: 0,
    phase: "idle",
    x: 50,
    y: 50
  });
  const [canScrollPreviewLeft, setCanScrollPreviewLeft] = useState(false);
  const [canScrollPreviewRight, setCanScrollPreviewRight] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token");
    if (token) {
      verifyMagicLink(token)
        .then((result) => {
          setToken(result.session.token);
          setAuth((current) => ({ ...current, token: result.session.token }));
          window.history.replaceState({}, "", window.location.pathname);
        })
        .catch((err: Error) => setError(err.message));
    }
  }, []);

  const normalizedQuery = query.trim().toLowerCase();

  useEffect(() => {
    if (!auth.token) return;
    refresh(libraryPage, normalizedQuery);
  }, [auth.token, libraryPage, normalizedQuery]);

  useEffect(() => {
    if (!auth.token) return;
    const interval = window.setInterval(() => {
      setCockatielDirection(Math.random() < 0.15 ? "reverse" : "forward");
      setCockatielRun((run) => run + 1);
    }, 90_000);
    return () => window.clearInterval(interval);
  }, [auth.token]);

  useEffect(() => {
    return () => {
      stopAudioPlayback(false);
    };
  }, []);

  const visibleVocab = [...vocab]
    .sort((left, right) => new Date(right.item.created_at).getTime() - new Date(left.item.created_at).getTime());

  useEffect(() => {
    setLibraryPage((current) => Math.min(current, pageCount(vocabTotal, libraryPageSize)));
  }, [vocabTotal]);

  const currentQuizCard = sessionDeck[sessionIndex];
  const sessionActive = Boolean(currentQuizCard);
  const sessionProgress = sessionDeck.length > 0 ? Math.round((sessionIndex / sessionDeck.length) * 100) : 0;
  const libraryPageCount = pageCount(vocabTotal, libraryPageSize);
  const paginatedVocab = visibleVocab;
  const rawImportCards = parseBulkImport(bulkText);
  const parsedImportCards = enrichedCards ?? rawImportCards;

  function updateImportPreviewScrollState() {
    const scroller = importPreviewRef.current;
    if (!scroller) {
      setCanScrollPreviewLeft(false);
      setCanScrollPreviewRight(false);
      return;
    }
    const edgeTolerance = 24;
    const maxScrollLeft = scroller.scrollWidth - scroller.clientWidth;
    setCanScrollPreviewLeft(scroller.scrollLeft > edgeTolerance);
    setCanScrollPreviewRight(maxScrollLeft - scroller.scrollLeft > edgeTolerance);
  }

  function scrollImportPreview(direction: "left" | "right") {
    const scroller = importPreviewRef.current;
    if (!scroller) return;
    scroller.scrollBy({
      left: direction === "right" ? scroller.clientWidth * 0.82 : -scroller.clientWidth * 0.82,
      behavior: "smooth"
    });
  }

  useEffect(() => {
    if (addMode !== "bulk") return;
    const frame = window.requestAnimationFrame(updateImportPreviewScrollState);
    window.addEventListener("resize", updateImportPreviewScrollState);
    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("resize", updateImportPreviewScrollState);
    };
  }, [addMode, parsedImportCards.length]);

  useEffect(() => {
    if (addMode !== "bulk") return;
    const frame = window.requestAnimationFrame(() => {
      const scroller = importPreviewRef.current;
      if (scroller) {
        scroller.scrollLeft = 0;
      }
      updateImportPreviewScrollState();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [addMode, parsedImportCards.length]);

  async function refresh(page = libraryPage, searchQuery = normalizedQuery) {
    const requestID = refreshRequestIDRef.current + 1;
    refreshRequestIDRef.current = requestID;
    setIsRefreshing(true);
    setVocab([]);
    setEditingID("");
    try {
      const [vocabResponse, dueResponse, statsResponse, jobsResponse] = await Promise.all([
        listVocab({
          limit: libraryPageSize,
          offset: (page - 1) * libraryPageSize,
          q: searchQuery
        }),
        listDue(),
        getReviewStats(),
        listNotificationJobs()
      ]);
      if (requestID !== refreshRequestIDRef.current) return;
      setVocab(vocabResponse.items);
      setDue(dueResponse.items);
      setVocabTotal(vocabResponse.total);
      setStats(statsResponse.stats);
      setJobs(jobsResponse.items);
      setError("");
    } catch (err) {
      if (requestID !== refreshRequestIDRef.current) return;
      handleRequestError(err);
    } finally {
      if (requestID === refreshRequestIDRef.current) {
        setIsRefreshing(false);
      }
    }
  }

  function clearAuthenticatedState() {
    stopAudioPlayback();
    refreshRequestIDRef.current += 1;
    clearToken();
    setAuth((current) => ({ ...current, token: "", magicLink: undefined, magicMessage: undefined }));
    setVocab([]);
    setDue([]);
    setVocabTotal(0);
    setStats(emptyStats);
    setJobs([]);
    setEditingID("");
    setEditDraft(emptyForm);
    setSessionDeck([]);
    setSessionIndex(0);
    setSessionCorrectCount(0);
    setSessionWrongCount(0);
    setSessionSummary(null);
    setSelectedOptionID("");
    setPendingNextDue("");
    setIsRefreshing(false);
    setIsStartingReview(false);
    setIsSaving(false);
    setIsGrading(false);
    setIsUserMenuOpen(false);
  }

  function stopAudioPlayback(updateState = true) {
    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.src = "";
      audioRef.current = null;
    }
    if (updateState) {
      setPlayingAudioID("");
    }
  }

  async function handlePlayAudio(item: VocabItem) {
    if (playingAudioID === item.id) {
      stopAudioPlayback();
      return;
    }
    if (item.audio?.status !== "ready") return;
    stopAudioPlayback();
    let url = item.audio.url;
    if (!url) {
      try {
        url = (await getVocabAudioURL(item.id)).url;
      } catch {
        setError(`Could not load pronunciation URL for "${item.term}".`);
        return;
      }
    }
    try {
      const audio = new Audio(url);
      audioRef.current = audio;
      setPlayingAudioID(item.id);
      audio.addEventListener("ended", () => {
        if (audioRef.current === audio) {
          setPlayingAudioID("");
          audioRef.current = null;
        }
      });
      audio.addEventListener("error", () => {
        if (audioRef.current === audio) {
          setError(`Could not play pronunciation for "${item.term}".`);
          stopAudioPlayback();
        }
      });
      await audio.play();
      setError("");
    } catch {
      setError(`Could not play pronunciation for "${item.term}".`);
      stopAudioPlayback();
    }
  }

  function handleRequestError(error: unknown) {
    if (isUnauthorizedError(error)) {
      clearAuthenticatedState();
      setError("Session expired. Sign in again.");
      return true;
    }
    setError((error as Error).message);
    return false;
  }

  async function handleRequestLink(event: FormEvent) {
    event.preventDefault();
    try {
      const response = await requestMagicLink(auth.email);
      setAuth((current) => ({ ...current, magicLink: response.verification_url, magicMessage: response.message }));
      setError("");
    } catch (err) {
      handleRequestError(err);
    }
  }

  function applyCreatedCard(createdCard: VocabWithState) {
    const isDueNow = new Date(createdCard.state.next_due_at).getTime() <= Date.now();
    setVocab((current) => [createdCard, ...current].slice(0, libraryPageSize));
    setVocabTotal((current) => current + 1);
    if (isDueNow) {
      setDue((current) => [createdCard, ...current]);
    }
    setStats((current) => ({
      ...current,
      active_cards: current.active_cards + 1,
      due_now: current.due_now + (isDueNow ? 1 : 0)
    }));
  }

  async function handleCreateVocab(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    try {
      const response = await createVocab(form);
      if (response.skipped_duplicate) {
        setLastCreatedTerm("");
        setLastImportCount(0);
        setLastSkippedDuplicateTerm(response.item.term);
        setLastSkippedDuplicateCount(0);
        setError("");
        return;
      }
      const createdCard = { item: response.item, state: response.state };
      setForm(emptyForm);
      applyCreatedCard(createdCard);
      setLastCreatedTerm(response.item.term);
      setLastImportCount(0);
      setLastSkippedDuplicateTerm("");
      setLastSkippedDuplicateCount(0);
      setError("");
      requestAnimationFrame(() => termInputRef.current?.focus());
    } catch (err) {
      handleRequestError(err);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleBulkImport(event: FormEvent) {
    event.preventDefault();
    if (parsedImportCards.length === 0) return;

    setIsSaving(true);
    let importedCount = 0;
    let skippedCount = 0;
    try {
      for (const card of parsedImportCards) {
        const response = await createVocab({
          term: card.term,
          meaning: card.meaning,
          chinese: card.chinese,
          example_sentence: card.example_sentence,
          part_of_speech: card.part_of_speech,
          notes: ""
        });
        if (response.skipped_duplicate) {
          skippedCount += 1;
          continue;
        }
        applyCreatedCard({ item: response.item, state: response.state });
        importedCount += 1;
      }
      handleBulkTextChange("");
      setEnrichedCards(null);
      setEnrichmentError("");
      setLastCreatedTerm("");
      setLastImportCount(importedCount);
      setLastSkippedDuplicateTerm("");
      setLastSkippedDuplicateCount(skippedCount);
      setError("");
    } catch (err) {
      setLastImportCount(importedCount);
      setLastSkippedDuplicateCount(skippedCount);
      handleRequestError(err);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleAutocompleteBulk() {
    const inputSnapshot = bulkText;
    const cards = parseBulkImport(inputSnapshot);
    if (cards.length === 0) return;

    setIsEnriching(true);
    setEnrichmentError("");
    try {
      const items: AutocompleteItem[] = cards.map(({ term, meaning, chinese, example_sentence, part_of_speech }) => ({
        term,
        meaning,
        chinese,
        example_sentence,
        part_of_speech
      }));
      const response = await autocompleteVocab(items);
      if (bulkTextRef.current !== inputSnapshot) return;
      const mergedCards = mergeAutocompleteResults(cards, response.items);
      const mergedText = formatBulkImportCards(mergedCards);
      bulkTextRef.current = mergedText;
      setBulkText(mergedText);
      setEnrichedCards(mergedCards);
    } catch (err) {
      if (bulkTextRef.current !== inputSnapshot) return;
      if (handleRequestError(err)) return;
      setEnrichmentError(`${(err as Error).message}. Manual import still works with the details currently shown.`);
    } finally {
      setIsEnriching(false);
    }
  }

  async function handleSaveEdit(event: FormEvent) {
    event.preventDefault();
    if (!editingID) return;
    setIsSaving(true);
    try {
      await updateVocab(editingID, editDraft);
      setEditingID("");
      setEditDraft(emptyForm);
      await refresh();
    } catch (err) {
      handleRequestError(err);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleArchive(id: string) {
    setIsSaving(true);
    try {
      await deleteVocab(id);
      if (editingID === id) {
        setEditingID("");
        setEditDraft(emptyForm);
      }
      await refresh();
    } catch (err) {
      handleRequestError(err);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleGrade(id: string, grade: "again" | "hard" | "good" | "easy") {
    setIsGrading(true);
    try {
      await gradeReview(id, grade);
      await refresh();
    } catch (err) {
      handleRequestError(err);
    } finally {
      setIsGrading(false);
    }
  }

  async function prepareReviewSession(): Promise<PreparedReviewSession | null> {
    setIsStartingReview(true);
    try {
      const [freshDueResponse, vocabResponse, statsResponse] = await Promise.all([
        listDue(),
        listVocab({ limit: 100, offset: 0 }),
        getReviewStats()
      ]);

      if (freshDueResponse.items.length === 0) {
        setDue(freshDueResponse.items);
        setStats(statsResponse.stats);
        setError("");
        return null;
      }

      const deck = buildQuizDeck(freshDueResponse.items, vocabResponse.items, reviewSessionSize);
      if (deck.length === 0) {
        setDue(freshDueResponse.items);
        setStats(statsResponse.stats);
        setError("Start Review needs at least one due card with a meaning and one other active card with a meaning.");
        return null;
      }
      setError("");
      return {
        deck,
        dueItems: freshDueResponse.items,
        stats: statsResponse.stats
      };
    } catch (err) {
      handleRequestError(err);
      return null;
    } finally {
      setIsStartingReview(false);
    }
  }

  function startPreparedReviewSession(prepared: PreparedReviewSession) {
    setDue(prepared.dueItems);
    setStats(prepared.stats);
    setSessionDeck(prepared.deck);
    setSessionIndex(0);
    setSessionCorrectCount(0);
    setSessionWrongCount(0);
    setSessionSummary(null);
    setSelectedOptionID("");
    setError("");
  }

  async function handleStartReviewClick(event: MouseEvent<HTMLButtonElement>) {
    const rect = event.currentTarget.getBoundingClientRect();
    const x = ((rect.left + rect.width / 2) / window.innerWidth) * 100;
    const y = ((rect.top + rect.height / 2) / window.innerHeight) * 100;

    setReviewTransition((current) => ({
      key: current.key + 1,
      phase: "exit",
      x,
      y
    }));

    const prepared = await prepareReviewSession();
    if (!prepared) {
      setReviewTransition((current) => ({ ...current, phase: "idle" }));
      return;
    }

    window.setTimeout(() => {
      setReviewTransition((current) => ({ ...current, phase: "wipe" }));
    }, 260);

    window.setTimeout(() => {
      startPreparedReviewSession(prepared);
    }, 980);

    window.setTimeout(() => {
      setReviewTransition((current) => ({ ...current, phase: "idle" }));
    }, 1680);
  }

  function endReviewSession() {
    stopAudioPlayback();
    setSessionDeck([]);
    setSessionIndex(0);
    setSessionCorrectCount(0);
    setSessionWrongCount(0);
    setSelectedOptionID("");
    setPendingNextDue("");
  }

  function returnToReviewHome() {
    stopAudioPlayback();
    setSessionSummary(null);
    setActiveSection("review");
  }

  async function handleQuizAnswer(option: QuizOption) {
    if (!currentQuizCard || selectedOptionID) return;
    setSelectedOptionID(option.id);
    setIsGrading(true);
    try {
      const grade: ReviewGrade = option.isCorrect ? "easy" : "again";
      const response = await gradeReview(currentQuizCard.card.item.id, grade);
      const reviewed = sessionIndex + 1;
      const correct = sessionCorrectCount + (option.isCorrect ? 1 : 0);
      const wrong = sessionWrongCount + (option.isCorrect ? 0 : 1);
      setSessionCorrectCount(correct);
      setSessionWrongCount(wrong);
      await refresh();
      setPendingNextDue(response.state.next_due_at);
      setError("");
    } catch (err) {
      handleRequestError(err);
    } finally {
      setIsGrading(false);
    }
  }

  function advanceQuizCard() {
    const reviewed = sessionIndex + 1;
    completeOrAdvanceQuiz(reviewed, sessionCorrectCount, sessionWrongCount, pendingNextDue);
  }

  function completeOrAdvanceQuiz(reviewed: number, correct: number, wrong: number, lastNextDue: string) {
    stopAudioPlayback();
    if (reviewed >= sessionDeck.length) {
      setSessionSummary({
        reviewed,
        correct,
        wrong,
        lastNextDue
      });
      endReviewSession();
      return;
    }
    setSessionIndex(reviewed);
    setSelectedOptionID("");
    setPendingNextDue("");
  }

  function startEditing(item: VocabItem) {
    setEditingID(item.id);
    setEditDraft(draftFromItem(item));
  }

  function handleBulkTextChange(value: string) {
    bulkTextRef.current = value;
    setBulkText(value);
    setEnrichedCards(null);
    setEnrichmentError("");
  }

  function openLibrary() {
    setQuery("");
    setLibraryPage(1);
    setIsUserMenuOpen(false);
    setActiveSection("library");
  }

  function handleSignOut() {
    clearAuthenticatedState();
    setError("");
  }

  if (!auth.token) {
    return (
      <main className="shell auth-shell">
        <section className="panel auth">
          <p className="eyebrow">Vocabulary review system</p>
          <h1>Sign in with email</h1>
          <form onSubmit={handleRequestLink} className="stack">
            <input
              type="email"
              placeholder="you@example.com"
              value={auth.email}
              onChange={(event) => setAuth((current) => ({ ...current, email: event.target.value }))}
            />
            <button type="submit">Create login link</button>
          </form>
          {auth.magicLink ? (
            <div className="callout">
              <p>Development magic link</p>
              <a href={auth.magicLink}>{auth.magicLink}</a>
            </div>
          ) : null}
          {!auth.magicLink && auth.magicMessage ? <p className="notice">{auth.magicMessage}</p> : null}
          {error ? <p className="error">{error}</p> : null}
        </section>
      </main>
    );
  }

  const isReviewFocused = activeSection === "review" && sessionActive;
  const isReviewExitingHome = activeSection === "review" && reviewTransition.phase !== "idle";
  const isReviewWiping = activeSection === "review" && reviewTransition.phase === "wipe";

  return (
    <main className={`app-shell mode-${activeSection}${isReviewFocused ? " is-review-focused" : ""}${isReviewExitingHome ? " is-review-transitioning" : ""}${isReviewWiping ? " is-review-wiping" : ""}`}>
      <div className="tokyo-sky" aria-hidden="true">
        <span className="skyline tower" />
        <span className="skyline block-a" />
        <span className="skyline block-b" />
        <span className="skyline block-c" />
        <span className="neon-orb orb-cyan" />
        <span className="neon-orb orb-rose" />
      </div>
      {!isReviewFocused ? (
        <div
          key={cockatielRun}
          className={`cockatiel-runner ${cockatielDirection === "reverse" ? "reverse" : "forward"}`}
          aria-hidden="true"
        >
          <img src={cockatielWalk} alt="" draggable={false} />
        </div>
      ) : null}
      {activeSection !== "library" && !isReviewFocused ? (
        <header className="app-header">
          <button
            type="button"
            className="brand-button"
            onClick={() => {
              setIsUserMenuOpen(false);
              setActiveSection("review");
            }}
            aria-label="Go to review dashboard"
          >
            <span className="brand-mark" aria-hidden="true">✦</span>
            <span>VocabReview</span>
          </button>

          <nav className="header-actions" aria-label="Primary actions">
            <button
              type="button"
              className="outline-action"
              onClick={openLibrary}
            >
              <span aria-hidden="true">▦</span>
              Manage Cards
            </button>
            <button
              type="button"
              className="primary-action"
              onClick={() => {
                setIsUserMenuOpen(false);
                setAddMode("bulk");
                setActiveSection("add");
              }}
            >
              <span aria-hidden="true">+</span>
              Quick Add
            </button>
            <div className="user-menu">
              <button
                type="button"
                className="user-avatar"
                aria-label="Open user menu"
                aria-expanded={isUserMenuOpen}
                onClick={() => setIsUserMenuOpen((isOpen) => !isOpen)}
              >
                <span aria-hidden="true">☰</span>
                Account
              </button>
              {isUserMenuOpen ? (
                <div className="user-dropdown" role="menu">
                  <button
                    type="button"
                    className="sign-out-menu-item"
                    role="menuitem"
                    onPointerDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      handleSignOut();
                    }}
                    onClick={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      handleSignOut();
                    }}
                  >
                    Sign out
                  </button>
                </div>
              ) : null}
            </div>
          </nav>
        </header>
      ) : null}

      <section className="workspace page-viewport">
        {activeSection === "review" ? (
          <section className="review-dashboard page-panel">
            {sessionActive ? (
              <article className="session-card">
                <div className="session-topline">
                  <span>
                    Card {sessionIndex + 1} of {sessionDeck.length}
                  </span>
                  <button type="button" className="ghost-button compact-button" onClick={endReviewSession} disabled={isGrading}>
                    End
                  </button>
                </div>
                <div className="session-progress" aria-label={`${sessionProgress}% complete`}>
                  <span style={{ width: `${sessionProgress}%` }} />
                </div>
                <div className="session-prompt" key={`prompt-${currentQuizCard.card.item.id}`}>
                  <p className="eyebrow">{partOfSpeechLabel(currentQuizCard.card.item)}</p>
                  <div className="session-term-line">
                    <h3>{currentQuizCard.card.item.term}</h3>
                    <AudioPlayButton item={currentQuizCard.card.item} isPlaying={playingAudioID === currentQuizCard.card.item.id} onPlay={handlePlayAudio} />
                  </div>
                </div>
                <div className="answer-options" aria-label="Meaning choices" key={`options-${currentQuizCard.card.item.id}`}>
                  {currentQuizCard.options.map((option, index) => {
                    const isSelected = selectedOptionID === option.id;
                    const showCorrect = Boolean(selectedOptionID) && option.isCorrect;
                    const showWrong = isSelected && !option.isCorrect;
                    return (
                      <button
                        key={option.id}
                        type="button"
                        className={[
                          "answer-option",
                          isSelected ? "selected" : "",
                          showCorrect ? "correct" : "",
                          showWrong ? "wrong" : ""
                        ].filter(Boolean).join(" ")}
                        onClick={() => handleQuizAnswer(option)}
                        disabled={Boolean(selectedOptionID) || isGrading}
                      >
                        <span>{String.fromCharCode(65 + index)}</span>
                        <strong>{option.text}</strong>
                      </button>
                    );
                  })}
                </div>
                {selectedOptionID ? (
                  <div className={`quiz-result ${currentQuizCard.options.find((option) => option.id === selectedOptionID)?.isCorrect ? "correct" : "wrong"}`}>
                    <div className="quiz-result-badge" aria-hidden="true">
                      {currentQuizCard.options.find((option) => option.id === selectedOptionID)?.isCorrect ? "✓" : "!"}
                    </div>
                    <div>
                      <strong>
                        {currentQuizCard.options.find((option) => option.id === selectedOptionID)?.isCorrect ? "Correct" : "Review again"}
                      </strong>
                      {currentQuizCard.card.item.chinese.trim() ? (
                        <span>{currentQuizCard.card.item.chinese}</span>
                      ) : null}
                      {currentQuizCard.options.find((option) => option.id === selectedOptionID)?.isCorrect ? (
                        currentQuizCard.card.item.example_sentence.trim() ? (
                          <small>{currentQuizCard.card.item.example_sentence}</small>
                        ) : null
                      ) : (
                        <>
                          <small>Correct answer: {answerText(currentQuizCard.card.item)}</small>
                          {currentQuizCard.card.item.example_sentence.trim() ? (
                            <small>{currentQuizCard.card.item.example_sentence}</small>
                          ) : null}
                        </>
                      )}
                    </div>
                  </div>
                ) : null}
                {selectedOptionID ? (
                  <button type="button" className="next-card-button" onClick={advanceQuizCard} disabled={isGrading}>
                    {sessionIndex + 1 >= sessionDeck.length ? "Show summary" : "Next word"}
                  </button>
                ) : null}
              </article>
            ) : sessionSummary ? (
              <section className="review-result-page" aria-label="Review session results">
                <div
                  className="review-accuracy-ring"
                  style={{ "--accuracy": `${reviewAccuracy(sessionSummary)}%` } as CSSProperties}
                  aria-label={`${reviewAccuracy(sessionSummary)} percent accuracy`}
                >
                  <span>{sessionSummary.correct}/{sessionSummary.reviewed}</span>
                  <small>correct</small>
                </div>

                <div className="review-result-copy">
                  <p className="eyebrow">Review complete</p>
                  <h1>{reviewResultMessage(sessionSummary)}</h1>
                </div>

                <div className="review-result-stats" aria-label="Review result breakdown">
                  <article>
                    <strong>{sessionSummary.reviewed}</strong>
                    <span>Reviewed</span>
                  </article>
                  <article>
                    <strong>{sessionSummary.correct}</strong>
                    <span>Correct</span>
                  </article>
                  <article>
                    <strong>{sessionSummary.wrong}</strong>
                    <span>Wrong</span>
                  </article>
                  <article>
                    <strong>{reviewAccuracy(sessionSummary)}%</strong>
                    <span>Accuracy</span>
                  </article>
                </div>

                {sessionSummary.lastNextDue ? (
                  <p className="review-result-return">Last card returns {formatDate(sessionSummary.lastNextDue)}.</p>
                ) : null}

                <button type="button" className="review-result-button" onClick={returnToReviewHome}>
                  Back to Home
                </button>
              </section>
            ) : (
              <div className="home-layout">
                <article className="review-start-card">
                  <div
                    className={`review-icon${isReviewIconLoaded ? " is-loaded" : ""}`}
                    aria-hidden="true"
                    onAnimationEnd={(event) => {
                      if (event.animationName === "gentle-content-in") {
                        setIsReviewIconLoaded(true);
                      }
                    }}
                  >
                    📖
                  </div>
                  <div className="review-card-copy">
                    <h1>{stats.due_now === 0 ? "Clear desk." : "Ready when you are."}</h1>
                    <span>
                      {stats.due_now === 0 ? "No cards ready to review" : `${stats.due_now} card${stats.due_now === 1 ? "" : "s"} ready to review`}
                    </span>
                  </div>
                  {stats.due_now > 0 ? (
                    <button type="button" className="start-review-button" onClick={handleStartReviewClick} disabled={due.length === 0 || isStartingReview || reviewTransition.phase !== "idle"}>
                      {isStartingReview ? "Preparing..." : "Start Review"}
                    </button>
                  ) : null}
                </article>

                <div className="home-stats" aria-label="Review stats">
                  <article>
                    <strong>{stats.active_cards}</strong>
                    <span>Total Cards</span>
                  </article>
                  <article>
                    <strong>{stats.reviewed_today}</strong>
                    <span>Reviewed</span>
                  </article>
                  <article>
                    <strong>{stats.reviewed_7_days}</strong>
                    <span>7 day reviews</span>
                  </article>
                </div>
              </div>
            )}
          </section>
        ) : null}

        {activeSection === "add" ? (
          <section className="quick-add-page page-panel">
            <header className="add-page-header">
              <h1>{addMode === "single" ? "Quick add" : "Bulk import"}</h1>
            </header>

            <div className={`add-mode-toggle mode-${addMode}`} role="tablist" aria-label="Add mode">
              <span className="toggle-indicator" aria-hidden="true" />
              <button
                type="button"
                className={addMode === "single" ? "active" : ""}
                role="tab"
                aria-selected={addMode === "single"}
                onClick={() => setAddMode("single")}
              >
                Single card
              </button>
              <button
                type="button"
                className={addMode === "bulk" ? "active" : ""}
                role="tab"
                aria-selected={addMode === "bulk"}
                onClick={() => setAddMode("bulk")}
              >
                Bulk import
              </button>
            </div>

            <div className="add-form-shell">
              {addMode === "single" ? (
                <form className="add-form-panel anim-right" onSubmit={handleCreateVocab}>
                  <label className="field-label">
                    <span>Word</span>
                    <input
                      ref={termInputRef}
                      value={form.term}
                      placeholder="e.g. meticulous"
                      onChange={(event) => setForm({ ...form, term: event.target.value })}
                    />
                  </label>
                  <div className="form-divider" />
                  <div className="optional-grid">
                    <label className="field-label">
                      <span>Meaning</span>
                      <textarea value={form.meaning} placeholder="Short definition" onChange={(event) => setForm({ ...form, meaning: event.target.value })} />
                    </label>
                    <label className="field-label">
                      <span>Chinese</span>
                      <textarea value={form.chinese} placeholder="中文意思" onChange={(event) => setForm({ ...form, chinese: event.target.value })} />
                    </label>
                  </div>
                  <label className="field-label">
                    <span>Example sentence</span>
                    <textarea
                      value={form.example_sentence}
                      placeholder="Use it in context"
                      onChange={(event) => setForm({ ...form, example_sentence: event.target.value })}
                    />
                  </label>
                  <label className="field-label">
                    <span>Notes</span>
                    <textarea value={form.notes} placeholder="Memory hint or source" onChange={(event) => setForm({ ...form, notes: event.target.value })} />
                  </label>
                  <button type="submit" className="add-save-button" disabled={isSaving || !form.term.trim()}>
                    {isSaving ? "Saving..." : "Save Card"}
                  </button>
                  {lastCreatedTerm ? <p className="save-confirmation">Saved "{lastCreatedTerm}". Ready for the next one.</p> : null}
                  {lastSkippedDuplicateTerm ? <p className="save-confirmation skipped">Skipped duplicate "{lastSkippedDuplicateTerm}".</p> : null}
                </form>
              ) : (
                <form className="add-form-panel anim-left" onSubmit={handleBulkImport}>
                  <label className="field-label">
                    <span className="field-label-row">
                      <span>Paste your words</span>
                      <button
                        type="button"
                        className="gpt-badge"
                        disabled={isEnriching || rawImportCards.length === 0}
                        onClick={handleAutocompleteBulk}
                      >
                        <span className="gpt-dot" aria-hidden="true" />
                        {isEnriching ? "Working..." : "GPT Auto-complete"}
                      </button>
                    </span>
                    <textarea
                      className="bulk-textarea"
                      value={bulkText}
                      placeholder="abandon | to leave behind | 放棄 | She had to abandon the plan. | verb"
                      onChange={(event) => handleBulkTextChange(event.target.value)}
                    />
                  </label>
                  <div className="bulk-hint">
                    One card per line. Full format: <code>word | definition | 中文 | example sentence | part_of_speech</code>.
                  </div>
                  {parsedImportCards.length > 0 ? (
                    <div className="import-preview-frame">
                      {canScrollPreviewLeft ? (
                        <button
                          type="button"
                          className="preview-arrow preview-arrow-left"
                          aria-label="Slide preview left"
                          onClick={() => scrollImportPreview("left")}
                        >
                          <span aria-hidden="true">‹</span>
                        </button>
                      ) : null}
                      <div className="import-preview" ref={importPreviewRef} onScroll={updateImportPreviewScrollState}>
                        {parsedImportCards.slice(0, 12).map((card, index) => (
                          <article key={`${card.term}-${index}`} className="import-preview-card">
                            <strong>{card.term}</strong>
                            {card.part_of_speech ? <span className="pos-pill">{card.part_of_speech}</span> : null}
                            <span>{card.meaning || "Meaning can be added later."}</span>
                            {card.chinese ? <span>{card.chinese}</span> : null}
                            {card.example_sentence ? <span>{card.example_sentence}</span> : null}
                            {card.error ? <span className="form-error">{card.error}</span> : null}
                          </article>
                        ))}
                        {parsedImportCards.length > 12 ? <span className="import-preview-more">+ {parsedImportCards.length - 12} more</span> : null}
                      </div>
                      {canScrollPreviewRight ? (
                        <button
                          type="button"
                          className="preview-arrow preview-arrow-right"
                          aria-label="Slide preview right"
                          onClick={() => scrollImportPreview("right")}
                        >
                          <span aria-hidden="true">›</span>
                        </button>
                      ) : null}
                    </div>
                  ) : null}
                  {enrichmentError ? <p className="form-error">{enrichmentError}</p> : null}
                  <button type="submit" className="add-save-button" disabled={isSaving || parsedImportCards.length === 0}>
                    {isSaving ? "Saving..." : `Import ${parsedImportCards.length} Card${parsedImportCards.length === 1 ? "" : "s"}`}
                  </button>
                  {lastImportCount ? (
                    <p className="save-confirmation">
                      Imported {lastImportCount} card{lastImportCount === 1 ? "" : "s"}.
                      {lastSkippedDuplicateCount ? ` Skipped ${lastSkippedDuplicateCount} duplicate${lastSkippedDuplicateCount === 1 ? "" : "s"}.` : ""}
                    </p>
                  ) : null}
                  {!lastImportCount && lastSkippedDuplicateCount ? (
                    <p className="save-confirmation skipped">
                      Skipped {lastSkippedDuplicateCount} duplicate{lastSkippedDuplicateCount === 1 ? "" : "s"}.
                    </p>
                  ) : null}
                </form>
              )}
            </div>
          </section>
        ) : null}

        {activeSection === "library" ? (
          <section className="library-panel page-panel">
            <div className="manage-header">
              <button type="button" className="back-button" onClick={() => setActiveSection("review")} aria-label="Back to review dashboard">
                ←
              </button>
              <span className="brand-mark small-mark" aria-hidden="true">✦</span>
              <h1>Manage Cards</h1>
            </div>

            <div className="filters">
              <input
                className="search-input"
                value={query}
                placeholder="Search cards..."
                onChange={(event) => {
                  setQuery(event.target.value);
                  setLibraryPage(1);
                }}
              />
            </div>

            <div className="library" key={`library-${libraryPage}-${query}`}>
              {isRefreshing && paginatedVocab.length === 0 ? (
                <div className="library-empty-text">
                  <strong>Loading cards...</strong>
                </div>
              ) : !isRefreshing && vocabTotal === 0 ? (
                <div className="library-empty-text">
                  <strong>{stats.active_cards === 0 ? "No cards yet." : "No matching cards."}</strong>
                </div>
              ) : paginatedVocab.map(({ item, state }) => (
                <article className={editingID === item.id ? "library-card editing" : "library-card"} key={item.id}>
                  {editingID === item.id ? (
                    <form className="edit-form" onSubmit={handleSaveEdit}>
                      <label className="field-label">
                        <span>Word</span>
                        <input value={editDraft.term} onChange={(event) => setEditDraft({ ...editDraft, term: event.target.value })} />
                      </label>
                      <label className="field-label">
                        <span>Meaning</span>
                        <textarea value={editDraft.meaning} onChange={(event) => setEditDraft({ ...editDraft, meaning: event.target.value })} />
                      </label>
                      <label className="field-label">
                        <span>Chinese</span>
                        <textarea value={editDraft.chinese} onChange={(event) => setEditDraft({ ...editDraft, chinese: event.target.value })} />
                      </label>
                      <label className="field-label">
                        <span>Example sentence</span>
                        <textarea
                          value={editDraft.example_sentence}
                          onChange={(event) => setEditDraft({ ...editDraft, example_sentence: event.target.value })}
                        />
                      </label>
                      <label className="field-label">
                        <span>Notes</span>
                        <textarea value={editDraft.notes} onChange={(event) => setEditDraft({ ...editDraft, notes: event.target.value })} />
                      </label>
                      <div className="action-row">
                        <button type="submit" disabled={isSaving}>{isSaving ? "Saving..." : "Save changes"}</button>
                        <button type="button" className="ghost-button" onClick={() => setEditingID("")}>Cancel</button>
                      </div>
                    </form>
                  ) : (
                    <div className="library-row">
                      <div className="library-copy">
                        <div className="library-title-line">
                          <h2>{item.term}</h2>
                          <AudioPlayButton item={item} isPlaying={playingAudioID === item.id} onPlay={handlePlayAudio} />
                          <span className="part-of-speech-chip">{item.part_of_speech ? item.part_of_speech.replace(/_/g, " ") : "No part of speech"}</span>
                        </div>
                        <p>{item.meaning || "Meaning not added yet."}</p>
                        <p className="chinese-line">{item.chinese || "Chinese not added yet."}</p>
                        {item.example_sentence ? <small>{item.example_sentence}</small> : null}
                        {item.notes ? <small className="notes">Notes: {item.notes}</small> : null}
                      </div>
                      <span className="library-actions">
                        <button type="button" className="icon-button ghost-button" aria-label={`Edit ${item.term}`} onClick={() => startEditing(item)}>
                          ✎
                        </button>
                        <button type="button" className="icon-button danger-button" aria-label={`Archive ${item.term}`} disabled={isSaving} onClick={() => handleArchive(item.id)}>
                          ×
                        </button>
                      </span>
                    </div>
                  )}
                </article>
              ))}
            </div>
            <Pagination label="Active cards" page={libraryPage} totalPages={libraryPageCount} onPageChange={setLibraryPage} />
          </section>
        ) : null}

        {error ? <p className="error floating-error">{error}</p> : null}
      </section>
      {isReviewWiping ? (
        <div
          key={reviewTransition.key}
          className="review-transition-overlay"
          style={{
            "--review-wipe-x": `${reviewTransition.x}%`,
            "--review-wipe-y": `${reviewTransition.y}%`
          } as CSSProperties}
          aria-hidden="true"
        />
      ) : null}
    </main>
  );
}

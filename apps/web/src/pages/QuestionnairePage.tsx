import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  assessmentsApi,
  type QuestionNode,
  type ResponseRecord,
} from "../api/assessments";
import { RubricCard } from "../components/RubricCard";
import { ProgressBar } from "../components/ProgressBar";
import { useAutoSave } from "../hooks/useAutoSave";

// Per-question draft edits stored separately from saved responses.
interface Draft {
  level: number | null;
  freeText: string;
}

export function QuestionnairePage() {
  const { id: assessmentId } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [questions, setQuestions] = useState<QuestionNode[]>([]);
  const [responses, setResponses] = useState<Map<string, ResponseRecord>>(
    new Map(),
  );
  // Keyed by question slug: unsaved/in-progress edits for each question.
  const [drafts, setDrafts] = useState<Map<string, Draft>>(new Map());
  const [cursor, setCursor] = useState(0);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (!assessmentId) return;
    Promise.all([
      assessmentsApi.listQuestions(assessmentId),
      assessmentsApi.listResponses(assessmentId),
    ])
      .then(([qRes, rRes]) => {
        setQuestions(qRes.data);
        const map = new Map<string, ResponseRecord>();
        for (const r of rRes.data) map.set(r.question_slug, r);
        setResponses(map);

        // Start at first unanswered question.
        const firstUnanswered = qRes.data.findIndex((q) => !map.has(q.slug));
        setCursor(firstUnanswered === -1 ? 0 : firstUnanswered);
      })
      .catch((err: unknown) => {
        const msg =
          (err as { response?: { data?: { error?: string } } })?.response?.data
            ?.error ?? "Failed to load questionnaire. Please refresh.";
        setLoadError(msg);
      })
      .finally(() => setLoading(false));
  }, [assessmentId]);

  const currentQuestion: QuestionNode | undefined = questions[cursor];

  // Derive current level/freeText: draft takes precedence over saved response.
  const savedResponse = currentQuestion
    ? responses.get(currentQuestion.slug)
    : undefined;
  const draft = currentQuestion ? drafts.get(currentQuestion.slug) : undefined;
  const pendingLevel: number | null =
    draft?.level !== undefined ? draft.level : (savedResponse?.level ?? null);
  const freeText: string =
    draft?.freeText !== undefined
      ? draft.freeText
      : (savedResponse?.free_text ?? "");

  function setDraft(slug: string, patch: Partial<Draft>) {
    setDrafts((prev) => {
      const existing = prev.get(slug) ?? { level: null, freeText: "" };
      const next = new Map(prev);
      next.set(slug, { ...existing, ...patch });
      return next;
    });
    setSaveError(null);
  }

  const answered = responses.size;
  const total = questions.length;

  // Auto-save when level or freeText changes for current question.
  useAutoSave(async () => {
    if (!assessmentId || !currentQuestion || pendingLevel === null) return;
    setSaving(true);
    setSaveError(null);
    try {
      await assessmentsApi.upsertResponse(assessmentId, currentQuestion.slug, {
        level: pendingLevel,
        free_text: freeText || undefined,
      });
      setResponses((prev) => {
        const next = new Map(prev);
        next.set(currentQuestion.slug, {
          question_slug: currentQuestion.slug,
          level: pendingLevel,
          free_text: freeText || undefined,
          updated_at: new Date().toISOString(),
          assigned_role: currentQuestion.assigned_role,
        });
        return next;
      });
      // Clear draft after successful save.
      setDrafts((prev) => {
        const next = new Map(prev);
        next.delete(currentQuestion.slug);
        return next;
      });
    } catch {
      setSaveError("Failed to save. Will retry on next change.");
    } finally {
      setSaving(false);
    }
  }, [pendingLevel, freeText, assessmentId, currentQuestion?.slug]);

  function goTo(index: number) {
    if (index >= 0 && index < questions.length) {
      setCursor(index);
    }
  }

  async function handleSubmit() {
    if (!assessmentId) return;
    setSubmitting(true);
    try {
      await assessmentsApi.submit(assessmentId);
      navigate("/app/assessments");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data
          ?.error ?? "Submission failed.";
      setSaveError(msg);
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-32 text-sm text-slate-400">
        Loading questionnaire…
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="flex flex-col items-center justify-center py-32 gap-4">
        <p className="text-sm text-red-400">{loadError}</p>
        <button
          className="btn-ghost text-xs"
          onClick={() => navigate("/app/assessments")}
        >
          ← Back to assessments
        </button>
      </div>
    );
  }

  if (questions.length === 0) {
    return (
      <div className="text-sm text-slate-400">
        No questions assigned to you for this assessment.
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl">
      {/* Header */}
      <div className="mb-6 flex items-center justify-between gap-4">
        <button
          onClick={() => navigate("/app/assessments")}
          className="btn-ghost text-xs"
        >
          ← Back
        </button>
        <div className="flex-1">
          <ProgressBar total={total} answered={answered} label="Progress" />
        </div>
        <span className="whitespace-nowrap text-xs text-slate-400">
          {cursor + 1} / {total}
        </span>
      </div>

      {/* Question card */}
      <div className="card">
        <div className="mb-1 flex items-center gap-2">
          <span className="rounded bg-brand-900 px-2 py-0.5 text-xs font-medium text-brand-300">
            {currentQuestion.assigned_role}
          </span>
          {responses.has(currentQuestion.slug) && (
            <span className="text-xs text-green-400">✓ Saved</span>
          )}
          {saving && <span className="text-xs text-slate-500">Saving…</span>}
        </div>

        <p className="mt-3 text-base font-medium leading-relaxed text-slate-100">
          {currentQuestion.prompt}
        </p>

        {saveError && <p className="mt-2 text-xs text-red-400">{saveError}</p>}

        {/* Rubric */}
        <div className="mt-6">
          <RubricCard
            rubricLevels={currentQuestion.rubric_levels}
            selected={pendingLevel}
            onChange={(level) => setDraft(currentQuestion.slug, { level })}
          />
        </div>

        {/* Free text */}
        <div className="mt-4">
          <label className="label mb-1" htmlFor="free-text">
            Notes / evidence (optional)
          </label>
          <textarea
            id="free-text"
            ref={textareaRef}
            rows={3}
            className="input resize-none"
            value={freeText}
            onChange={(e) =>
              setDraft(currentQuestion.slug, { freeText: e.target.value })
            }
            placeholder="Add context, links, or evidence…"
          />
        </div>
      </div>

      {/* Navigation */}
      <div className="mt-6 flex items-center justify-between">
        <button
          className="btn-ghost"
          disabled={cursor === 0}
          onClick={() => goTo(cursor - 1)}
        >
          ← Previous
        </button>

        <div className="flex gap-2">
          {cursor < questions.length - 1 ? (
            <button className="btn-primary" onClick={() => goTo(cursor + 1)}>
              Next →
            </button>
          ) : (
            <button
              className="btn-primary"
              disabled={answered < total || submitting}
              onClick={handleSubmit}
            >
              {submitting
                ? "Submitting…"
                : answered < total
                  ? `${total - answered} unanswered`
                  : "Submit assessment"}
            </button>
          )}
        </div>
      </div>

      {/* Question nav dots */}
      <div className="mt-8 flex flex-wrap gap-1.5">
        {questions.map((q, i) => (
          <button
            key={q.slug}
            onClick={() => goTo(i)}
            title={q.slug}
            className={`h-2.5 w-2.5 rounded-full transition-colors ${
              i === cursor
                ? "bg-brand-400"
                : responses.has(q.slug)
                  ? "bg-green-600"
                  : "bg-surface-border"
            }`}
          />
        ))}
      </div>
    </div>
  );
}

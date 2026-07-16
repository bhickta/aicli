import type { CopyMetadata, QuestionMetadata, StudyCopyRecord, StudyQuestionRecord } from "../types";

export interface StudyQuestionFilters {
  query: string;
  subject: string;
  topic: string;
  paper: string;
  difficulty: string;
}

export interface StudyQuestionFilterOptions {
  subjects: string[];
  topics: string[];
  papers: string[];
  difficulties: string[];
}

interface CopyMetadataPayload {
  copy?: CopyMetadata;
}

export function copyMetadata(copy: StudyCopyRecord): CopyMetadata | null {
  const payload = parseMetadata<CopyMetadataPayload | CopyMetadata>(copy.metadata_json);
  if (!payload) return null;
  const wrapped = (payload as CopyMetadataPayload).copy;
  return wrapped || payload as CopyMetadata;
}

export function questionMetadata(question: StudyQuestionRecord): QuestionMetadata | null {
  return parseMetadata<QuestionMetadata>(question.metadata_json);
}

export function copyMetadataChips(copy: StudyCopyRecord): string[] {
  const meta = copyMetadata(copy);
  return compactUnique([
    meta?.paper || copy.paper,
    meta?.test_code || copy.test_code,
    meta?.coaching_institute,
    meta?.test_series,
    meta?.exam,
    meta?.year,
    ...(meta?.tags || []),
  ]).slice(0, 6);
}

export function questionMetadataChips(question: StudyQuestionRecord): string[] {
  const meta = questionMetadata(question);
  if (!meta) return compactUnique([question.marks ? `${question.marks} marks` : "", question.word_limit ? `${question.word_limit} words` : ""]);
  return compactUnique([
    meta.paper,
    meta.subject,
    meta.topic,
    meta.subtopic,
    meta.difficulty,
    meta.marks ? `${meta.marks} marks` : question.marks ? `${question.marks} marks` : "",
    meta.word_limit ? `${meta.word_limit} words` : question.word_limit ? `${question.word_limit} words` : "",
    ...(meta.tags || []),
  ]).slice(0, 8);
}

export function questionFilterOptions(questions: StudyQuestionRecord[]): StudyQuestionFilterOptions {
  return {
    subjects: uniqueField(questions, (question) => questionMetadata(question)?.subject),
    topics: uniqueField(questions, (question) => questionMetadata(question)?.topic),
    papers: uniqueField(questions, (question) => questionMetadata(question)?.paper),
    difficulties: uniqueField(questions, (question) => questionMetadata(question)?.difficulty),
  };
}

export function filterStudyQuestions(questions: StudyQuestionRecord[], filters: StudyQuestionFilters): StudyQuestionRecord[] {
  return questions.filter((question) => {
    const meta = questionMetadata(question);
    return matchesText(question, meta, filters.query)
      && matchesFilter(meta?.subject, filters.subject)
      && matchesFilter(meta?.topic, filters.topic)
      && matchesFilter(meta?.paper, filters.paper)
      && matchesFilter(meta?.difficulty, filters.difficulty);
  });
}

function parseMetadata<T>(json: string): T | null {
  if (!json?.trim()) return null;
  try {
    return JSON.parse(json) as T;
  } catch {
    return null;
  }
}

function matchesText(question: StudyQuestionRecord, meta: QuestionMetadata | null, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const haystack = [
    question.label,
    question.prompt_text,
    question.answer_text,
    meta ? JSON.stringify(meta) : "",
  ].join(" ").toLowerCase();
  return haystack.includes(needle);
}

function matchesFilter(value: string | undefined, filter: string): boolean {
  if (!filter) return true;
  return (value || "").toLowerCase() === filter.toLowerCase();
}

function uniqueField(questions: StudyQuestionRecord[], pick: (question: StudyQuestionRecord) => string | undefined): string[] {
  return compactUnique(questions.map(pick)).sort((a, b) => a.localeCompare(b));
}

function compactUnique(values: Array<string | undefined>): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const value of values) {
    const clean = (value || "").trim();
    const key = clean.toLowerCase();
    if (!clean || seen.has(key)) continue;
    seen.add(key);
    out.push(clean);
  }
  return out;
}

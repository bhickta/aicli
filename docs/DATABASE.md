# AICLI — Database Reference

## Overview

The Analyze domain uses a **SQLite** database (`analyze.db`) located at `{data_dir}/analyze.db`.

- **Thread safety:** WAL journal mode + per-thread connections via `threading.local()`
- **Timeout:** 30s connection timeout, 5000ms busy timeout
- **Row factory:** `sqlite3.Row` (dict-like access)

**Source files:**
- Schema creation: `aicli/domains/analyze/db/base.py`
- Page operations: `aicli/domains/analyze/db/pages.py`
- Answer operations: `aicli/domains/analyze/db/answers.py`
- Dimension operations: `aicli/domains/analyze/db/dimensions.py`
- Log/reset operations: `aicli/domains/analyze/db/logs.py`
- Repository (HTTP layer): `aicli/server/repositories/analyze_repository.py`

---

## Schema

### `pages`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PK AUTOINCREMENT | Unique page ID |
| `pdf_file` | TEXT | NOT NULL | Source PDF filename |
| `page_number` | INTEGER | NOT NULL | 1-based page number |
| `image_path` | TEXT | NOT NULL | Path to cached PNG image |
| `classification` | TEXT | NULL | cover / answer / continuation / evaluation / blank |
| `transcription` | TEXT | NULL | OCR-transcribed text |
| `processed` | INTEGER | DEFAULT 0 | 1 = transcription complete |
| | | UNIQUE(pdf_file, page_number) | |

### `answers`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PK AUTOINCREMENT | Unique answer ID |
| `pdf_file` | TEXT | NOT NULL | Source PDF filename |
| `candidate_name` | TEXT | NULL | Extracted from cover page |
| `upsc_id` | TEXT | NULL | UPSC registration ID |
| `test_code` | TEXT | NULL | Exam paper code |
| `question_number` | TEXT | NULL | e.g. "Q.1", "1(a)" |
| `question_text` | TEXT | NULL | Full question text |
| `question_directive` | TEXT | NULL | e.g. "Discuss", "Comment" |
| `word_limit` | INTEGER | NULL | If specified in question |
| `raw_text` | TEXT | NULL | Cleaned segmented answer text |
| `page_ids` | TEXT | NULL | JSON array of page IDs: `[1, 2, 3]` |
| `segmentation_done` | INTEGER | DEFAULT 0 | 1 = segmentation complete |

### `answer_dimensions`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PK AUTOINCREMENT | |
| `answer_id` | INTEGER | NOT NULL, FK → answers(id) | Which answer |
| `dimension_name` | TEXT | NOT NULL | e.g. "intro", "outro", "formatting" |
| `result_json` | TEXT | NOT NULL | JSON analysis result |
| `processed_at` | TEXT | NOT NULL | ISO timestamp |
| | | UNIQUE(answer_id, dimension_name) | |

### `dimension_aggregations`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PK AUTOINCREMENT | |
| `dimension_name` | TEXT | NOT NULL, UNIQUE | One aggregation per dimension |
| `aggregation_json` | TEXT | NOT NULL | Cross-PDF synthesis JSON |
| `generated_at` | TEXT | NOT NULL | ISO timestamp |
| `answer_count` | INTEGER | NOT NULL | How many answers were aggregated |

### `processing_log`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PK AUTOINCREMENT | |
| `pdf_file` | TEXT | NULL | Which PDF (NULL for system events) |
| `step` | TEXT | NOT NULL | Pipeline step identifier |
| `status` | TEXT | NOT NULL | "done", "error", "reset_from_N" |
| `error` | TEXT | NULL | Error message if any |
| `timestamp` | TEXT | NOT NULL | ISO timestamp |

---

## Common Queries (Copy-Paste Ready)

### Open the database

```bash
# From project root, assuming data dir is ./data
sqlite3 ./data/analyze.db -header -column
```

### Check overall status

```sql
-- How many PDFs, pages, answers?
SELECT 'PDFs' as metric, COUNT(DISTINCT pdf_file) as count FROM pages
UNION ALL
SELECT 'Pages', COUNT(*) FROM pages
UNION ALL
SELECT 'Transcribed', COUNT(*) FROM pages WHERE transcription IS NOT NULL
UNION ALL
SELECT 'Classified', COUNT(*) FROM pages WHERE classification IS NOT NULL
UNION ALL
SELECT 'Answers', COUNT(*) FROM answers
UNION ALL
SELECT 'Dimensions', COUNT(*) FROM answer_dimensions
UNION ALL
SELECT 'Aggregations', COUNT(*) FROM dimension_aggregations;
```

### Per-PDF progress

```sql
SELECT
  pdf_file,
  COUNT(*) as total_pages,
  SUM(CASE WHEN transcription IS NOT NULL THEN 1 ELSE 0 END) as transcribed,
  SUM(CASE WHEN classification IS NOT NULL THEN 1 ELSE 0 END) as classified
FROM pages
GROUP BY pdf_file
ORDER BY pdf_file;
```

### Find transcription errors

```sql
SELECT id, pdf_file, page_number, transcription
FROM pages
WHERE transcription LIKE '[TRANSCRIPTION_ERROR%';
```

### List answers for a PDF

```sql
SELECT id, question_number, question_directive,
       LENGTH(raw_text) as text_length, page_ids
FROM answers
WHERE pdf_file = 'example.pdf'
ORDER BY CAST(question_number AS INTEGER);
```

### Check dimension analysis coverage

```sql
SELECT
  a.pdf_file,
  a.question_number,
  GROUP_CONCAT(ad.dimension_name) as analyzed_dimensions
FROM answers a
LEFT JOIN answer_dimensions ad ON a.id = ad.answer_id
GROUP BY a.id
ORDER BY a.pdf_file, CAST(a.question_number AS INTEGER);
```

### Find unanalyzed answers for a dimension

```sql
SELECT a.id, a.pdf_file, a.question_number
FROM answers a
WHERE a.id NOT IN (
  SELECT answer_id FROM answer_dimensions
  WHERE dimension_name = 'intro'
    AND result_json NOT LIKE '%"error":%'
)
ORDER BY a.pdf_file;
```

### View dimension result for a specific answer

```sql
SELECT dimension_name, result_json
FROM answer_dimensions
WHERE answer_id = 42;
```

### View aggregation results

```sql
SELECT dimension_name, answer_count,
       json_extract(aggregation_json, '$.patterns[0].pattern_name') as top_pattern
FROM dimension_aggregations;
```

### Check processing log

```sql
SELECT * FROM processing_log
ORDER BY timestamp DESC
LIMIT 20;
```

### Count errors by step

```sql
SELECT step, COUNT(*) as error_count
FROM processing_log
WHERE status = 'error'
GROUP BY step;
```

---

## Reset Operations

The `reset_from_step()` method cascades deletions from a given step downward:

| Reset from step | What gets cleared |
|-----------------|-------------------|
| 1 | DELETE all pages (cascade: everything) |
| 2 | SET transcription = NULL, processed = 0 |
| 3 | SET classification = NULL |
| 4 | DELETE answers |
| 5 | DELETE answer_dimensions |
| 6 | DELETE dimension_aggregations |

### Manual reset examples

```sql
-- Reset everything for re-processing
DELETE FROM dimension_aggregations;
DELETE FROM answer_dimensions;
DELETE FROM answers;
UPDATE pages SET classification = NULL;
UPDATE pages SET transcription = NULL, processed = 0;

-- Reset just one PDF
DELETE FROM answer_dimensions WHERE answer_id IN (SELECT id FROM answers WHERE pdf_file = 'test.pdf');
DELETE FROM answers WHERE pdf_file = 'test.pdf';
UPDATE pages SET classification = NULL WHERE pdf_file = 'test.pdf';
UPDATE pages SET transcription = NULL, processed = 0 WHERE pdf_file = 'test.pdf';

-- Retry all transcription errors
UPDATE pages SET transcription = NULL, processed = 0
WHERE transcription LIKE '[TRANSCRIPTION_ERROR%';
```

---

## Enabled Dimensions

Configured in `domains/analyze/prompts.yaml` under `dimensions:`:

| Dimension | What it analyzes |
|-----------|-----------------|
| `intro` | Introduction pattern (definition, context, statistical...) |
| `outro` | Conclusion pattern (recommendation, way_forward...) |
| `transition` | Transition phrases between sections |
| `formatting` | Numbering, underlines, boxes, headers |
| `diagram` | Diagram usage (maps, flowcharts, tables) |
| `selling_point` | Most impressive/distinctive element |

To add a new dimension, add an entry under `dimensions:` in `prompts.yaml` with `enabled: true` and a `prompt:` field. No code changes needed.

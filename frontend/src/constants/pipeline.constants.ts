/** Shared pipeline step definitions — used by PipelineRunner, AnalyzeBanner, etc. */

export interface PipelineStepDef {
  id: number
  name: string
  fullname: string
}

export const PIPELINE_STEPS: PipelineStepDef[] = [
  { id: 1, name: 'Images', fullname: 'PDF → Page Images' },
  { id: 2, name: 'OCR', fullname: 'OCR Transcription' },
  { id: 3, name: 'Classify', fullname: 'Page Classification' },
  { id: 4, name: 'Segment', fullname: 'Answer Segmentation' },
  { id: 5, name: 'Analyze', fullname: 'Dimension Analysis' },
  { id: 6, name: 'Aggregate', fullname: 'Cross-PDF Aggregation' },
  { id: 7, name: 'Report', fullname: 'Report Generation' },
]

/** Default per-step reasoning toggles. */
export const DEFAULT_STEP_REASONING: Record<number, boolean> = {
  2: false,
  3: false,
  4: false,
  5: true,
  6: true,
  7: true,
}

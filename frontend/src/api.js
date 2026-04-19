const API_BASE = 'http://localhost:8765'

export async function fetchStatus() {
  const res = await fetch(`${API_BASE}/api/analyze/status`)
  return res.json()
}

export async function fetchPdfs() {
  const res = await fetch(`${API_BASE}/api/analyze/pdfs`)
  return res.json()
}

export async function fetchPages(pdf) {
  const res = await fetch(`${API_BASE}/api/analyze/pdfs/${pdf.id}/pages`)
  return res.json()
}

export async function fetchAnswers(pdf) {
  const res = await fetch(`${API_BASE}/api/analyze/pdfs/${pdf.id}/answers`)
  return res.json()
}

export async function fetchDimensions(answerId) {
  const res = await fetch(`${API_BASE}/api/analyze/answers/${answerId}/dimensions`)
  return res.json()
}

export async function fetchAggregations() {
  const res = await fetch(`${API_BASE}/api/analyze/aggregate`)
  const data = await res.json()
  return data ? Object.keys(data).map(k => ({ dimension_name: k, ...data[k] })) : []
}

export async function resetPipeline(step) {
  const res = await fetch(`${API_BASE}/api/analyze/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ step }),
  })
  return res.json()
}

export async function retryErrors() {
  const res = await fetch(`${API_BASE}/api/analyze/retry-errors`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
  return res.json()
}

export async function runPipeline(config) {
  const res = await fetch(`${API_BASE}/api/analyze/run`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
  return res.json()
}

export function createStream() {
  return new EventSource(`${API_BASE}/api/analyze/stream`)
}

export async function deletePdf(pdfFile) {
  const res = await fetch(`${API_BASE}/api/analyze/pdfs/${encodeURIComponent(pdfFile)}`, {
    method: 'DELETE'
  })
  return res.json()
}

export function imageUrl(pdfFile, pageNumber) {
  const paddedPage = String(pageNumber).padStart(4, '0')
  // PDF file name is the directory name under images/
  const pdfName = pdfFile.replace(/\.pdf$/i, '')
  return `${API_BASE}/api/analyze/images/${encodeURIComponent(pdfName)}/page_${paddedPage}.png`
}

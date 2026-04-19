/** Class-based API client for the Analyze domain — all HTTP in one place. */
import { API_BASE } from '../constants/api.constants'

export class AnalyzeApiClient {
  private readonly base = `${API_BASE}/api/analyze`

  async fetchStatus(): Promise<any> {
    return this.get('/status')
  }

  async fetchPdfs(): Promise<any[]> {
    return this.get('/pdfs')
  }

  async fetchPages(pdf: { id: number }): Promise<any[]> {
    return this.get(`/pdfs/${pdf.id}/pages`)
  }

  async fetchAnswers(pdf: { id: number }): Promise<any[]> {
    return this.get(`/pdfs/${pdf.id}/answers`)
  }

  async fetchDimensions(answerId: number): Promise<any[]> {
    return this.get(`/answers/${answerId}/dimensions`)
  }

  async fetchAggregations(): Promise<any[]> {
    const data = await this.get('/aggregate')
    if (!data) return []
    if (Array.isArray(data)) return data
    return Object.keys(data).map(k => ({ dimension_name: k, ...data[k] }))
  }

  async resetPipeline(step: number): Promise<any> {
    return this.post('/reset', { step })
  }

  async retryErrors(): Promise<any> {
    return this.post('/retry-errors', {})
  }

  async runPipeline(config: Record<string, unknown>): Promise<any> {
    return this.post('/run', config)
  }

  async deletePdf(pdfFile: string): Promise<any> {
    const res = await fetch(`${this.base}/pdfs/${encodeURIComponent(pdfFile)}`, {
      method: 'DELETE',
    })
    return this.handleResponse(res)
  }

  async uploadPdfs(files: FileList): Promise<any> {
    const formData = new FormData()
    for (let i = 0; i < files.length; i++) {
      formData.append('files', files[i])
    }
    const res = await fetch(`${this.base}/upload`, {
      method: 'POST',
      body: formData,
    })
    return this.handleResponse(res)
  }

  createStream(): EventSource {
    return new EventSource(`${this.base}/stream`)
  }

  imageUrl(pdfFile: string, pageNumber: number): string {
    const paddedPage = String(pageNumber).padStart(4, '0')
    const pdfName = pdfFile.replace(/\.pdf$/i, '')
    return `${this.base}/images/${encodeURIComponent(pdfName)}/page_${paddedPage}.png`
  }

  // ── Private Helpers ──────────────────────────────────────────

  private async get(path: string): Promise<any> {
    const res = await fetch(`${this.base}${path}`)
    return this.handleResponse(res)
  }

  private async post(path: string, body: Record<string, unknown>): Promise<any> {
    const res = await fetch(`${this.base}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    return this.handleResponse(res)
  }

  private async handleResponse(res: Response): Promise<any> {
    if (!res.ok) {
      const body = await res.json().catch(() => ({ detail: 'Request failed' }))
      throw new Error(body.detail ?? `HTTP ${res.status}`)
    }
    return res.json()
  }
}

/** Singleton instance for convenience. */
export const analyzeApi = new AnalyzeApiClient()

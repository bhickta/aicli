import { API_BASE } from '../constants/api.constants'

export class FSApiClient {
  private readonly base = `${API_BASE}/api/fs`

  async listDir(path: string = "/"): Promise<any> {
    const res = await fetch(`${this.base}/list?path=${encodeURIComponent(path)}`)
    return this.handleResponse(res)
  }

  async getHome(): Promise<any> {
    const res = await fetch(`${this.base}/home`)
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

export const fsApi = new FSApiClient()

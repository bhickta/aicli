import { API_BASE } from '../constants/api.constants'

export class SettingsApiClient {
  private readonly base = `${API_BASE}/api/settings`

  async fetchSettings(): Promise<any> {
    const res = await fetch(this.base)
    return this.handleResponse(res)
  }

  async updateSettings(config: any): Promise<any> {
    const res = await fetch(this.base, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    })
    return this.handleResponse(res)
  }

  async fetchProviders(): Promise<any> {
    const res = await fetch(`${this.base}/providers`)
    return this.handleResponse(res)
  }

  async fetchModels(): Promise<any> {
    const res = await fetch(`${this.base}/models`)
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

export const settingsApi = new SettingsApiClient()

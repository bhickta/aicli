import { ref, onMounted, onUnmounted } from 'vue'
import { analyzeApi } from '../api/AnalyzeApiClient'

export function useAnalyzeStatus() {
  const status = ref<any>(null)
  const pdfs = ref<any[]>([])
  const loading = ref(false)
  let intervalId: any = null

  async function refreshStatus() {
    try {
      status.value = await analyzeApi.fetchStatus()
    } catch (e: any) {
      console.warn('API not reachable:', e.message)
    }
  }

  async function loadPdfs() {
    try {
      pdfs.value = await analyzeApi.fetchPdfs()
    } catch (e) {
      pdfs.value = []
    }
  }

  async function refreshAll() {
    loading.value = true
    await Promise.all([refreshStatus(), loadPdfs()])
    loading.value = false
  }

  onMounted(() => {
    refreshAll()
    intervalId = setInterval(refreshStatus, 10000)
  })

  onUnmounted(() => {
    if (intervalId) clearInterval(intervalId)
  })

  return {
    status,
    pdfs,
    loading,
    refreshAll,
    refreshStatus,
    loadPdfs
  }
}

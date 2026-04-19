import { ref, computed } from 'vue'

export function usePageInspector(pages: any[]) {
  const inspectedPage = ref<any>(null)
  const inspectorTab = ref('transcribe')

  const isFirstPage = computed(() => {
    if (!inspectedPage.value || !pages.length) return true
    return pages[0].id === inspectedPage.value.id
  })

  const isLastPage = computed(() => {
    if (!inspectedPage.value || !pages.length) return true
    return pages[pages.length - 1].id === inspectedPage.value.id
  })

  function openPageInspector(page: any) {
    inspectedPage.value = page
    inspectorTab.value = 'transcribe'
  }

  function closePageInspector() {
    inspectedPage.value = null
  }

  function nextPage() {
    if (!inspectedPage.value) return
    const idx = pages.findIndex(p => p.id === inspectedPage.value.id)
    if (idx < pages.length - 1) inspectedPage.value = pages[idx + 1]
  }

  function prevPage() {
    if (!inspectedPage.value) return
    const idx = pages.findIndex(p => p.id === inspectedPage.value.id)
    if (idx > 0) inspectedPage.value = pages[idx - 1]
  }

  return {
    inspectedPage,
    inspectorTab,
    isFirstPage,
    isLastPage,
    openPageInspector,
    closePageInspector,
    nextPage,
    prevPage
  }
}

import { ref, computed, type Ref } from 'vue'

export function usePageInspector(pages: Ref<any[]>) {
  const inspectedPage = ref<any>(null)
  const inspectorTab = ref('transcribe')

  const isFirstPage = computed(() => {
    if (!inspectedPage.value || !pages.value.length) return true
    return pages.value[0].id === inspectedPage.value.id
  })

  const isLastPage = computed(() => {
    if (!inspectedPage.value || !pages.value.length) return true
    return pages.value[pages.value.length - 1].id === inspectedPage.value.id
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
    const idx = pages.value.findIndex(p => p.id === inspectedPage.value.id)
    if (idx < pages.value.length - 1) inspectedPage.value = pages.value[idx + 1]
  }

  function prevPage() {
    if (!inspectedPage.value) return
    const idx = pages.value.findIndex(p => p.id === inspectedPage.value.id)
    if (idx > 0) inspectedPage.value = pages.value[idx - 1]
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

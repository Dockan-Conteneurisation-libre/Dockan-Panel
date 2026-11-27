// Loads a GET endpoint into a ref, with loading and error states.
import { onMounted, ref, shallowRef, type Ref, type ShallowRef } from 'vue'
import { api } from './api'

export interface Loaded<T> {
  data: ShallowRef<T | null>
  loading: Ref<boolean>
  error: Ref<string>
  reload: () => Promise<void>
}

export function useLoad<T>(path: () => string, immediate = true): Loaded<T> {
  const data = shallowRef<T | null>(null)
  const loading = ref(false)
  const error = ref('')
  async function reload(): Promise<void> {
    loading.value = true
    try {
      data.value = await api.get<T>(path())
      error.value = ''
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }
  if (immediate) onMounted(reload)
  return { data, loading, error, reload }
}

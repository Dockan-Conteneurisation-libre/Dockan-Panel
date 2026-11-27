// Promise-based confirmation dialog (no native window.confirm).
import { reactive } from 'vue'

export const dialog = reactive({
  open: false,
  text: '',
  danger: false,
  resolve: null as ((ok: boolean) => void) | null,
})

export function confirmAction(text: string, danger = true): Promise<boolean> {
  dialog.resolve?.(false)
  return new Promise((resolve) => {
    Object.assign(dialog, { open: true, text, danger, resolve })
  })
}

export function closeDialog(ok: boolean): void {
  const r = dialog.resolve
  Object.assign(dialog, { open: false, resolve: null })
  r?.(ok)
}

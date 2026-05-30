import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type UiState = {
  theme: 'light' | 'dark'
  collapsed: boolean
  setTheme: (theme: 'light' | 'dark') => void
  toggleCollapsed: () => void
}

export const useUiStore = create<UiState>()(
  persist(
    (set) => ({
      theme: 'light',
      collapsed: false,
      setTheme: (theme) => set({ theme }),
      toggleCollapsed: () => set((state) => ({ collapsed: !state.collapsed })),
    }),
    { name: 'yunshu-admin-ui' },
  ),
)

import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import type { Key } from './i18n'
import type { IconName } from './lib/icons'

export interface NavItem { path: string; label: Key; icon: IconName; group: Key }

export const nav: NavItem[] = [
  { path: '/', label: 'nav.overview', icon: 'overview', group: 'nav.groupRuntime' },
  { path: '/containers', label: 'nav.containers', icon: 'box', group: 'nav.groupRuntime' },
  { path: '/images', label: 'nav.images', icon: 'layers', group: 'nav.groupRuntime' },
  { path: '/volumes', label: 'nav.volumes', icon: 'disk', group: 'nav.groupRuntime' },
  { path: '/networks', label: 'nav.networks', icon: 'network', group: 'nav.groupRuntime' },
  { path: '/stacks', label: 'nav.stacks', icon: 'stack', group: 'nav.groupApps' },
  { path: '/store', label: 'nav.store', icon: 'store', group: 'nav.groupApps' },
  { path: '/system', label: 'nav.system', icon: 'cpu', group: 'nav.groupHost' },
  { path: '/security', label: 'nav.security', icon: 'shield', group: 'nav.groupHost' },
]

const routes: RouteRecordRaw[] = [
  { path: '/', component: () => import('./views/OverviewView.vue') },
  { path: '/containers', component: () => import('./views/ContainersView.vue') },
  { path: '/containers/:store/:name', component: () => import('./views/ContainerView.vue'), props: true },
  { path: '/images', component: () => import('./views/ImagesView.vue') },
  { path: '/volumes', component: () => import('./views/VolumesView.vue') },
  { path: '/networks', component: () => import('./views/NetworksView.vue') },
  { path: '/stacks', component: () => import('./views/StacksView.vue') },
  { path: '/store', component: () => import('./views/StoreView.vue') },
  { path: '/system', component: () => import('./views/SystemView.vue') },
  { path: '/security', component: () => import('./views/SecurityView.vue') },
  { path: '/:rest(.*)*', redirect: '/' },
]

// Hash history: the Go server needs no route list and old bookmarks to
// "/?view=..." still land on the panel.
export const router = createRouter({ history: createWebHashHistory(), routes })

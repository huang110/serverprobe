/**
 * 路由配置与登录守卫。
 */
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/agents'
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true, title: '登录' }
  },
  {
    path: '/agents',
    name: 'agent-list',
    component: () => import('@/views/AgentList.vue'),
    meta: { title: 'Agent 列表' }
  },
  {
    path: '/agents/:id',
    name: 'agent-detail',
    component: () => import('@/views/AgentDetail.vue'),
    meta: { title: 'Agent 详情' }
  },
  {
    path: '/alert/rules',
    name: 'alert-rules',
    component: () => import('@/views/AlertRules.vue'),
    meta: { title: '告警规则' }
  },
  {
    path: '/alert/records',
    name: 'alert-records',
    component: () => import('@/views/AlertRecords.vue'),
    meta: { title: '告警记录' }
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/agents'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 })
})

// 全局前置守卫：未登录跳转登录页
router.beforeEach((to) => {
  const authStore = useAuthStore()
  document.title = to.meta.title
    ? `${to.meta.title} · ServerProbe`
    : 'ServerProbe 监控系统'

  if (!to.meta.public && !authStore.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  // 已登录用户访问登录页时直接进入主页
  if (to.name === 'login' && authStore.isAuthenticated) {
    return { name: 'agent-list' }
  }

  return true
})

export default router

import { createRouter, createWebHistory } from 'vue-router'

import { getSession } from '@/services/api'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'movies',
      component: () => import('@/views/MovieCatalogView.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
    },
    {
      path: '/movies',
      redirect: '/',
    },
    {
      path: '/movies/:movieId',
      name: 'movie-detail',
      component: () => import('@/views/MovieDetailView.vue'),
      props: true,
    },
    {
      path: '/booking/:showtimeId',
      name: 'booking-create',
      component: () => import('@/views/BookingCreateView.vue'),
      props: true,
      meta: { requiresAuth: true },
    },
    {
      path: '/bookings',
      name: 'my-bookings',
      component: () => import('@/views/MyBookingsView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/bookings/history',
      name: 'booking-history',
      component: () => import('@/views/MyBookingsView.vue'),
      props: { history: true },
      meta: { requiresAuth: true },
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('@/views/AdminDashboardView.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
    {
      path: '/admin/showtimes/new',
      name: 'admin-showtime-create',
      component: () => import('@/views/AdminShowtimeCreateView.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.name !== 'login' && !to.meta.requiresAuth) return true

  try {
    const session = await getSession()
    if (to.meta.requiresAdmin && session.user.role !== 'ADMIN') {
      return { name: 'movies', replace: true }
    }
    return to.name === 'login' ? { name: 'movies', replace: true } : true
  } catch {
    return to.meta.requiresAuth ? { name: 'login', replace: true } : true
  }
})

export default router

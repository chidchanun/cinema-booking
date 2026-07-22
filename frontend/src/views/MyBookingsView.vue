<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { CalendarDays, ChevronLeft, ChevronRight, Clock3, LoaderCircle, Ticket } from '@lucide/vue'

import SiteHeader from '@/components/SiteHeader.vue'
import { ApiError } from '@/services/api'
import { listMyBookings, type Booking } from '@/services/bookings'
import { listMovies } from '@/services/movies'
import {
  listPendingReservations,
  type PendingReservation,
} from '@/services/pendingReservations'

const bookings = ref<Booking[]>([])
const loading = ref(true)
const errorMessage = ref('')
const pendingReservations = ref<PendingReservation[]>([])
const movieTitles = ref<Record<string, string>>({})
const movieFilter = ref('')
const dateFilter = ref('')
const page = ref(1)
const total = ref(0)
const totalPages = ref(1)
const pageSize = 10

const visibleBookings = computed(() => {
  const now = Date.now()
  return bookings.value.filter((booking) => {
    const isHistory = booking.status === 'CANCELLED' || new Date(booking.showtime_start).getTime() < now
    if (isHistory) return false
    return !isHistory
  })
})
const movieOptions = computed(() => {
  const options = new Map<string, string>()
  for (const [id, title] of Object.entries(movieTitles.value)) {
    options.set(id, title)
  }
  for (const reservation of pendingReservations.value) {
    if (reservation.movie_id) options.set(reservation.movie_id, reservation.movie_title)
  }
  return [...options.entries()]
    .map(([id, title]) => ({ id, title }))
    .sort((left, right) => left.title.localeCompare(right.title, 'th'))
})
const selectedMovieTitle = computed(
  () => movieOptions.value.find((movie) => movie.id === movieFilter.value)?.title,
)
const visiblePendingReservations = computed(() =>
  pendingReservations.value.filter((reservation) => {
    if (
      movieFilter.value &&
      reservation.movie_id !== movieFilter.value &&
      reservation.movie_title !== selectedMovieTitle.value
    ) return false
    return !dateFilter.value || localDate(reservation.showtime_start) === dateFilter.value
  }),
)

function localDate(value: string): string {
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 10)
}

function clearFilters(): void {
  movieFilter.value = ''
  dateFilter.value = ''
}

function bookingDateFilters(): { from: string; to?: string } {
  if (!dateFilter.value) return { from: new Date().toISOString() }
  return {
    from: new Date(`${dateFilter.value}T00:00:00`).toISOString(),
    to: new Date(`${dateFilter.value}T23:59:59.999`).toISOString(),
  }
}

function formatDate(value: string): string {
  const dateTime = new Intl.DateTimeFormat('th-TH', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
  return `${dateTime} น.`
}

function formatPrice(booking: Booking): string {
  return new Intl.NumberFormat('th-TH', {
    style: 'currency',
    currency: booking.currency || 'THB',
  }).format(booking.price)
}

async function loadBookings(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    pendingReservations.value = listPendingReservations()
    const [bookingResponse, movieResponse] = await Promise.all([
      listMyBookings({
        page: page.value,
        limit: pageSize,
        movieID: movieFilter.value,
        ...bookingDateFilters(),
      }),
      listMovies('', undefined, 100),
    ])
    bookings.value = bookingResponse.data
    total.value = bookingResponse.total
    totalPages.value = Math.max(bookingResponse.total_pages, 1)
    movieTitles.value = Object.fromEntries(
      movieResponse.data.map((movie) => [movie.id, movie.title]),
    )
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'ไม่สามารถโหลดรายการจองได้'
  } finally {
    loading.value = false
  }
}

onMounted(loadBookings)
watch([movieFilter, dateFilter], () => {
  page.value = 1
  void loadBookings()
})

async function changePage(nextPage: number): Promise<void> {
  if (nextPage < 1 || nextPage > totalPages.value || nextPage === page.value) return
  page.value = nextPage
  await loadBookings()
}
</script>

<template>
  <main class="bookings-page">
    <SiteHeader section="ตั๋วของฉัน" />
    <section class="bookings-shell">
      <header class="page-heading">
        <p>MY BOOKINGS</p>
        <h1>ตั๋วของฉัน</h1>
      </header>

      <section class="booking-filters" aria-label="ตัวกรองตั๋ว">
        <label>ภาพยนตร์<select v-model="movieFilter"><option value="">ทุกเรื่อง</option><option v-for="movie in movieOptions" :key="movie.id" :value="movie.id">{{ movie.title }}</option></select></label>
        <label>วันที่ฉาย<input v-model="dateFilter" type="date" /></label>
        <button type="button" :disabled="!movieFilter && !dateFilter" @click="clearFilters">ล้างตัวกรอง</button>
      </section>

      <section v-if="visiblePendingReservations.length" class="pending-section" aria-labelledby="pending-title">
        <div class="pending-heading">
          <div>
            <p>WAITING FOR PAYMENT</p>
            <h2 id="pending-title">รายการรอชำระเงิน</h2>
          </div>
          <span>ที่นั่งจะถูกปล่อยอัตโนมัติเมื่อหมดเวลา</span>
        </div>
        <article
          v-for="reservation in visiblePendingReservations"
          :key="reservation.showtime_id"
          class="pending-row"
        >
          <div><span>ภาพยนตร์</span><strong>{{ reservation.movie_title }}</strong></div>
          <div><span>ที่นั่ง</span><strong>{{ reservation.locks.map((lock) => lock.seat_code).join(', ') }}</strong></div>
          <div><span>Hall</span><strong>{{ reservation.hall_name }}</strong></div>
          <div><span><Clock3 :size="14" /> ชำระก่อน</span><strong>{{ formatDate(reservation.expires_at) }}</strong></div>
          <RouterLink class="pay-link" :to="`/booking/${reservation.showtime_id}`">ชำระเงิน</RouterLink>
        </article>
      </section>

      <div v-if="loading" class="status"><LoaderCircle class="spinner" :size="28" /> กำลังโหลดรายการจอง</div>
      <div v-else-if="errorMessage" class="status error" role="alert">
        <p>{{ errorMessage }}</p>
        <button type="button" @click="loadBookings">ลองอีกครั้ง</button>
      </div>
      <div v-else-if="visibleBookings.length === 0 && visiblePendingReservations.length === 0" class="status empty">
        <Ticket :size="34" />
        <h2>{{ movieFilter || dateFilter ? 'ไม่พบตั๋วตามตัวกรอง' : 'ยังไม่มีตั๋วที่กำลังจะมาถึง' }}</h2>
      </div>
      <div v-else class="booking-list">
        <article v-for="booking in visibleBookings" :key="booking.id" class="booking-row">
          <div class="booking-code"><span>BOOKING CODE</span><strong>{{ booking.booking_code }}</strong></div>
          <div><span>ภาพยนตร์</span><strong>{{ movieTitles[booking.movie_id] ?? 'ไม่พบชื่อภาพยนตร์' }}</strong></div>
          <div><span>ที่นั่ง</span><strong>{{ booking.seat_code }}</strong></div>
          <div><span>โรง</span><strong>{{ booking.hall_name }}</strong></div>
          <div class="showtime"><span><CalendarDays :size="14" /> รอบฉาย</span><strong>{{ formatDate(booking.showtime_start) }}</strong></div>
          <div><span>ยอดชำระ</span><strong>{{ formatPrice(booking) }}</strong></div>
        </article>
      </div>
      <nav v-if="total > 0" class="pagination" aria-label="หน้ารายการตั๋ว">
        <span>ทั้งหมด {{ total }} ใบ</span>
        <div><button type="button" title="หน้าก่อนหน้า" :disabled="page <= 1" @click="changePage(page - 1)"><ChevronLeft :size="17" /></button><strong>หน้า {{ page }} / {{ totalPages }}</strong><button type="button" title="หน้าถัดไป" :disabled="page >= totalPages" @click="changePage(page + 1)"><ChevronRight :size="17" /></button></div>
      </nav>
    </section>
  </main>
</template>

<style scoped>
.bookings-page { min-height: 100vh; color: #27231f; background: #f4f3f0; }
.bookings-shell { width: min(1050px, calc(100% - 48px)); margin: 0 auto; padding: 50px 0 70px; }
.page-heading { padding-bottom: 26px; border-bottom: 1px solid #d0cbc4; }
.page-heading p { margin: 0 0 8px; color: #936f25; font-size: 13px; font-weight: 800; }
.page-heading h1 { margin: 0; font-family: Georgia, "Times New Roman", serif; font-size: 40px; font-weight: 500; letter-spacing: 0; }
.booking-filters { display: flex; align-items: end; gap: 12px; margin-top: 22px; padding: 14px 16px; background: #fff; border: 1px solid #d8d3cc; }
.booking-filters label { display: grid; min-width: min(260px, 34vw); gap: 6px; color: #706a63; font-size: 13px; font-weight: 700; }
.booking-filters select, .booking-filters input { width: 100%; height: 40px; padding: 0 10px; color: #302c28; background: #fff; border: 1px solid #c9c3bb; border-radius: 3px; }
.booking-filters button { min-height: 40px; padding: 0 14px; color: #393530; background: #fff; border: 1px solid #c9c3bb; border-radius: 3px; cursor: pointer; }
.booking-filters button:disabled { cursor: not-allowed; opacity: .5; }
.pending-section { margin-top: 24px; padding-bottom: 22px; border-bottom: 1px solid #d0cbc4; }
.pending-heading { display: flex; align-items: end; justify-content: space-between; gap: 20px; margin-bottom: 12px; }
.pending-heading p { margin: 0 0 5px; color: #936f25; font-size: 12px; font-weight: 800; }
.pending-heading h2 { margin: 0; font-size: 21px; }
.pending-heading > span { color: #756f68; font-size: 13px; }
.pending-row { display: grid; grid-template-columns: 1.25fr .8fr .65fr 1.35fr auto; gap: 18px; align-items: center; padding: 18px 20px; background: #fff8e7; border: 1px solid #d8bd78; border-left: 4px solid #a87c24; border-radius: 4px; }
.pending-row > div { display: grid; gap: 5px; min-width: 0; }
.pending-row span { display: flex; align-items: center; gap: 5px; color: #7a6c51; font-size: 13px; }
.pending-row strong { overflow-wrap: anywhere; font-size: 15px; }
.pay-link { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; padding: 0 17px; color: #fff; background: #8b681f; border-radius: 4px; font-weight: 750; text-decoration: none; white-space: nowrap; }
.booking-list { display: grid; gap: 10px; margin-top: 24px; }
.pagination { display: flex; min-height: 52px; align-items: center; justify-content: space-between; gap: 16px; margin-top: 12px; padding: 8px 12px; color: #716b64; font-size: 13px; background: #eeece8; border: 1px solid #d6d1ca; }
.pagination > div { display: flex; align-items: center; gap: 10px; }
.pagination strong { min-width: 88px; color: #3b3733; text-align: center; }
.pagination button { display: grid; width: 34px; height: 34px; place-items: center; color: #393530; background: #fff; border: 1px solid #c8c2ba; border-radius: 3px; cursor: pointer; }
.pagination button:disabled { color: #aaa39b; background: #e5e2dd; cursor: not-allowed; }
.booking-row { display: grid; grid-template-columns: 1.25fr 1.1fr .55fr .65fr 1.35fr .7fr; gap: 16px; align-items: center; padding: 20px; background: #fff; border: 1px solid #d8d3cc; border-left: 4px solid #b48a36; border-radius: 4px; }
.booking-row > div { display: grid; gap: 5px; min-width: 0; }
.booking-row span { display: flex; align-items: center; gap: 5px; color: #7a746d; font-size: 13px; }
.booking-row strong { overflow-wrap: anywhere; font-size: 16px; }
.booking-code strong { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
.status { display: flex; min-height: 220px; align-items: center; justify-content: center; gap: 10px; color: #6f6963; }
.status.empty, .status.error { flex-direction: column; text-align: center; }
.status h2, .status p { margin: 0; }
.status button { min-height: 38px; padding: 0 16px; color: #fff; background: #2c2926; border: 0; border-radius: 4px; cursor: pointer; }
.error { color: #982e27; }
.spinner { animation: spin .85s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) {
  .bookings-shell { width: calc(100% - 28px); padding-top: 34px; }
  .booking-filters { align-items: stretch; flex-direction: column; }
  .booking-filters label { width: 100%; min-width: 0; }
  .booking-row { grid-template-columns: 1fr 1fr; }
  .booking-code, .showtime { grid-column: 1 / -1; }
  .pending-heading { align-items: start; flex-direction: column; }
  .pending-row { grid-template-columns: 1fr 1fr; }
  .pending-row > div:first-child, .pay-link { grid-column: 1 / -1; }
}
@media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
</style>

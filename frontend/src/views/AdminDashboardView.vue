<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  Activity,
  CalendarCheck,
  CalendarPlus,
  Film,
  LoaderCircle,
  Pencil,
  Plus,
  Search,
  Trash2,
  X,
} from '@lucide/vue'

import SiteHeader from '@/components/SiteHeader.vue'
import { ApiError } from '@/services/api'
import {
  createMovie,
  deleteMovie,
  getTMDBMovie,
  listAdminBookings,
  listAdminMovies,
  listAuditLogs,
  searchTMDBMovies,
  updateMovie,
  type AdminBooking,
  type AuditLog,
  type MovieInput,
  type TMDBMovie,
} from '@/services/admin'
import type { Movie } from '@/types/movie'

type AdminTab = 'movies' | 'bookings' | 'audit'

const activeTab = ref<AdminTab>('movies')
const movies = ref<Movie[]>([])
const bookings = ref<AdminBooking[]>([])
const auditLogs = ref<AuditLog[]>([])
const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const search = ref('')
const bookingStatus = ref('')
const bookingMovieID = ref('')
const bookingUserID = ref('')
const bookingFrom = ref('')
const bookingTo = ref('')
const bookingFilterUsers = ref<AdminBooking['user'][]>([])
const auditAction = ref('')
const showMovieForm = ref(false)
const editingMovieID = ref<string | null>(null)
const tmdbQuery = ref('')
const tmdbResults = ref<TMDBMovie[]>([])
const searchingTMDB = ref(false)
const tmdbError = ref('')
const selectingTMDBMovieID = ref<number | null>(null)
let tmdbSearchTimer: ReturnType<typeof setTimeout> | undefined
let tmdbSearchController: AbortController | undefined

const movieForm = reactive<MovieInput>({
  title: '',
  description: '',
  duration_minutes: 120,
  poster_url: '',
  is_active: true,
})

const activeMovies = computed(() => movies.value.filter((movie) => movie.is_active).length)
const confirmedBookings = computed(
  () => bookings.value.filter((booking) => booking.status === 'BOOKED').length,
)
const revenue = computed(() =>
  bookings.value
    .filter((booking) => booking.status === 'BOOKED')
    .reduce((total, booking) => total + booking.price, 0),
)

function bookingDateBoundary(value: string, endOfDay = false): string {
  if (!value) return ''
  return new Date(`${value}T${endOfDay ? '23:59:59.999' : '00:00:00.000'}`).toISOString()
}

function currentBookingFilters() {
  return {
    status: bookingStatus.value,
    movieID: bookingMovieID.value,
    userID: bookingUserID.value,
    from: bookingDateBoundary(bookingFrom.value),
    to: bookingDateBoundary(bookingTo.value, true),
  }
}

function errorText(error: unknown, fallback: string): string {
  return error instanceof ApiError ? error.message : fallback
}

async function loadDashboard(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    const [movieResponse, bookingResponse, auditResponse] = await Promise.all([
      listAdminMovies(search.value),
      listAdminBookings(),
      listAuditLogs(),
    ])
    movies.value = movieResponse.data
    bookings.value = bookingResponse.data
    bookingFilterUsers.value = Array.from(
      new Map(bookingResponse.data.map((booking) => [booking.user.id, booking.user])).values(),
    )
    auditLogs.value = auditResponse.data
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถโหลดข้อมูลผู้ดูแลระบบได้')
  } finally {
    loading.value = false
  }
}

async function loadMovies(): Promise<void> {
  errorMessage.value = ''
  try {
    movies.value = (await listAdminMovies(search.value)).data
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถโหลดรายการภาพยนตร์ได้')
  }
}

async function loadBookings(): Promise<void> {
  errorMessage.value = ''
  try {
    bookings.value = (await listAdminBookings(currentBookingFilters())).data
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถโหลดรายการจองได้')
  }
}

async function loadAuditLogs(): Promise<void> {
  errorMessage.value = ''
  try {
    auditLogs.value = (await listAuditLogs(auditAction.value)).data
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถโหลด Audit Logs ได้')
  }
}

function clearBookingFilters(): void {
  bookingStatus.value = ''
  bookingMovieID.value = ''
  bookingUserID.value = ''
  bookingFrom.value = ''
  bookingTo.value = ''
  void loadBookings()
}

function resetMovieForm(): void {
  editingMovieID.value = null
  Object.assign(movieForm, {
    title: '',
    description: '',
    duration_minutes: 120,
    poster_url: '',
    is_active: true,
  })
  tmdbQuery.value = ''
  tmdbResults.value = []
  tmdbError.value = ''
}

async function searchTMDB(): Promise<void> {
  if (!tmdbQuery.value.trim() || searchingTMDB.value) return
  tmdbSearchController?.abort()
  tmdbSearchController = new AbortController()
  searchingTMDB.value = true
  tmdbError.value = ''
  try {
    tmdbResults.value = (
      await searchTMDBMovies(tmdbQuery.value, tmdbSearchController.signal)
    ).data.slice(0, 8)
    if (tmdbResults.value.length === 0) tmdbError.value = 'ไม่พบภาพยนตร์จาก TMDB'
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    tmdbError.value = errorText(error, 'ไม่สามารถค้นหาภาพยนตร์จาก TMDB ได้')
  } finally {
    searchingTMDB.value = false
  }
}

watch(tmdbQuery, (query) => {
  if (tmdbSearchTimer) clearTimeout(tmdbSearchTimer)
  tmdbSearchController?.abort()

  if (query.trim().length < 2) {
    tmdbResults.value = []
    tmdbError.value = ''
    searchingTMDB.value = false
    return
  }

  tmdbSearchTimer = setTimeout(() => void searchTMDB(), 400)
})

async function selectTMDBMovie(movie: TMDBMovie): Promise<void> {
  if (selectingTMDBMovieID.value !== null) return
  selectingTMDBMovieID.value = movie.id
  tmdbError.value = ''
  try {
    const details = await getTMDBMovie(movie.id)
    movieForm.title = details.title
    movieForm.description = details.overview
    movieForm.poster_url = details.poster_url
    if (details.runtime && details.runtime > 0) movieForm.duration_minutes = details.runtime
    tmdbResults.value = []
    tmdbQuery.value = details.title
  } catch (error) {
    tmdbError.value = errorText(error, 'ไม่สามารถดึงรายละเอียดภาพยนตร์จาก TMDB ได้')
  } finally {
    selectingTMDBMovieID.value = null
  }
}

function openCreateMovie(): void {
  resetMovieForm()
  showMovieForm.value = true
}

function openEditMovie(movie: Movie): void {
  editingMovieID.value = movie.id
  Object.assign(movieForm, {
    title: movie.title,
    description: movie.description,
    duration_minutes: movie.duration_minutes,
    poster_url: movie.poster_url ?? '',
    is_active: movie.is_active,
  })
  showMovieForm.value = true
}

function closeMovieForm(): void {
  if (saving.value) return
  showMovieForm.value = false
  resetMovieForm()
}

async function saveMovie(): Promise<void> {
  if (saving.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    if (editingMovieID.value) {
      await updateMovie(editingMovieID.value, { ...movieForm })
    } else {
      await createMovie({ ...movieForm })
    }
    saving.value = false
    closeMovieForm()
    await loadMovies()
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถบันทึกภาพยนตร์ได้')
  } finally {
    saving.value = false
  }
}

async function removeMovie(movie: Movie): Promise<void> {
  if (!window.confirm(`ลบภาพยนตร์ “${movie.title}” หรือไม่?`)) return
  errorMessage.value = ''
  try {
    await deleteMovie(movie.id)
    await loadMovies()
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถลบภาพยนตร์ได้')
  }
}

async function toggleMovie(movie: Movie): Promise<void> {
  errorMessage.value = ''
  try {
    await updateMovie(movie.id, { is_active: !movie.is_active })
    await loadMovies()
  } catch (error) {
    errorMessage.value = errorText(error, 'ไม่สามารถเปลี่ยนสถานะภาพยนตร์ได้')
  }
}

function formatDate(value: string): string {
  const dateTime = new Intl.DateTimeFormat('th-TH', { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(value),
  )
  return `${dateTime} น.`
}

function formatMoney(value: number, currency = 'THB'): string {
  return new Intl.NumberFormat('th-TH', { style: 'currency', currency }).format(value)
}

onMounted(loadDashboard)
onBeforeUnmount(() => {
  if (tmdbSearchTimer) clearTimeout(tmdbSearchTimer)
  tmdbSearchController?.abort()
})
</script>

<template>
  <main class="admin-page">
    <SiteHeader section="ผู้ดูแลระบบ" />

    <div class="admin-layout">
      <header class="admin-heading">
        <div>
          <p>ADMINISTRATION</p>
          <h1>ศูนย์จัดการระบบ</h1>
        </div>
        <div class="heading-actions">
          <RouterLink class="secondary-button" to="/admin/showtimes/new"><CalendarPlus :size="18" /> เพิ่มรอบฉาย</RouterLink>
          <button v-if="activeTab === 'movies'" class="primary-button" type="button" @click="openCreateMovie"><Plus :size="18" /> เพิ่มภาพยนตร์</button>
        </div>
      </header>

      <section class="metrics" aria-label="ข้อมูลสรุป">
        <div><Film :size="20" /><span>ภาพยนตร์ที่เปิดแสดง</span><strong>{{ activeMovies }}</strong></div>
        <div><CalendarCheck :size="20" /><span>การจองที่ยืนยัน</span><strong>{{ confirmedBookings }}</strong></div>
        <div><Activity :size="20" /><span>ยอดจองรวม</span><strong>{{ formatMoney(revenue) }}</strong></div>
      </section>

      <nav class="admin-tabs" aria-label="เมนูจัดการ">
        <button :class="{ active: activeTab === 'movies' }" type="button" @click="activeTab = 'movies'">ภาพยนตร์</button>
        <button :class="{ active: activeTab === 'bookings' }" type="button" @click="activeTab = 'bookings'">รายการจอง</button>
        <button :class="{ active: activeTab === 'audit' }" type="button" @click="activeTab = 'audit'">Audit log</button>
      </nav>

      <p v-if="errorMessage" class="admin-error" role="alert">{{ errorMessage }}</p>
      <div v-if="loading" class="loading-state"><LoaderCircle class="spinner" :size="28" /> กำลังโหลดข้อมูล</div>

      <section v-else-if="activeTab === 'movies'" class="admin-content">
        <form class="toolbar" role="search" @submit.prevent="loadMovies">
          <Search :size="18" />
          <input v-model="search" type="search" placeholder="ค้นหาภาพยนตร์" aria-label="ค้นหาภาพยนตร์" />
          <button type="submit">ค้นหา</button>
        </form>
        <div class="table-wrap">
          <table>
            <thead><tr><th>ภาพยนตร์</th><th>ความยาว</th><th>สถานะ</th><th>อัปเดตล่าสุด</th><th><span class="sr-only">คำสั่ง</span></th></tr></thead>
            <tbody>
              <tr v-for="movie in movies" :key="movie.id">
                <td><div class="movie-cell"><div class="poster"><img v-if="movie.poster_url" :src="movie.poster_url" alt="" /><Film v-else :size="18" /></div><div><strong>{{ movie.title }}</strong><span>{{ movie.description || 'ไม่มีคำอธิบาย' }}</span></div></div></td>
                <td>{{ movie.duration_minutes }} นาที</td>
                <td><button class="status-toggle" :class="{ inactive: !movie.is_active }" type="button" @click="toggleMovie(movie)">{{ movie.is_active ? 'เปิดแสดง' : 'ปิดแสดง' }}</button></td>
                <td>{{ formatDate(movie.updated_at) }}</td>
                <td><div class="row-actions"><button type="button" title="แก้ไข" @click="openEditMovie(movie)"><Pencil :size="17" /></button><button class="danger" type="button" title="ลบ" @click="removeMovie(movie)"><Trash2 :size="17" /></button></div></td>
              </tr>
              <tr v-if="movies.length === 0"><td class="empty-row" colspan="5">ไม่พบภาพยนตร์</td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else-if="activeTab === 'bookings'" class="admin-content">
        <form class="toolbar booking-filter" @submit.prevent="loadBookings">
          <label>สถานะ<select v-model="bookingStatus"><option value="">ทั้งหมด</option><option value="BOOKED">ชำระแล้ว</option><option value="CANCELLED">ยกเลิก</option></select></label>
          <label>ภาพยนตร์<select v-model="bookingMovieID"><option value="">ทุกเรื่อง</option><option v-for="movie in movies" :key="movie.id" :value="movie.id">{{ movie.title }}</option></select></label>
          <label>ผู้ใช้<select v-model="bookingUserID"><option value="">ทุกคน</option><option v-for="user in bookingFilterUsers" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option></select></label>
          <label>ตั้งแต่<input v-model="bookingFrom" type="date" /></label>
          <label>ถึง<input v-model="bookingTo" type="date" :min="bookingFrom" /></label>
          <button type="submit"><Search :size="17" /> กรอง</button>
          <button class="clear-filter" type="button" @click="clearBookingFilters">ล้าง</button>
        </form>
        <div class="table-wrap">
          <table>
            <thead><tr><th>รหัสจอง</th><th>ลูกค้า</th><th>ภาพยนตร์</th><th>รอบฉาย / ที่นั่ง</th><th>ยอดชำระ</th><th>สถานะ</th></tr></thead>
            <tbody>
              <tr v-for="booking in bookings" :key="booking.id"><td><strong class="code">{{ booking.booking_code }}</strong></td><td><strong>{{ booking.user.name }}</strong><span class="subtext">{{ booking.user.email }}</span></td><td>{{ booking.movie.title }}</td><td>{{ formatDate(booking.showtime_start) }}<span class="subtext">{{ booking.hall_name }} · {{ booking.seat_code }}</span></td><td>{{ formatMoney(booking.price, booking.currency) }}</td><td><span class="badge" :class="booking.status.toLowerCase()">{{ booking.status === 'CANCELLED' ? 'ยกเลิก' : 'ชำระแล้ว' }}</span></td></tr>
              <tr v-if="bookings.length === 0"><td class="empty-row" colspan="6">ไม่พบรายการจอง</td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else class="admin-content">
        <div class="toolbar audit-filter">
          <label>เหตุการณ์<select v-model="auditAction" @change="loadAuditLogs"><option value="">ทั้งหมด</option><option value="BOOKING_CONFIRMED">Booking Success</option><option value="BOOKING_TIMEOUT">Booking Timeout</option><option value="SEAT_RELEASED">Seat Released</option><option value="SYSTEM_ERROR">System Error</option></select></label>
        </div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>เวลา</th><th>เหตุการณ์</th><th>การดำเนินการ</th><th>ผู้ดำเนินการ</th><th>ระดับ</th></tr></thead>
            <tbody>
              <tr v-for="log in auditLogs" :key="log.id"><td>{{ formatDate(log.occurred_at) }}</td><td><strong>{{ log.event_type }}</strong><span class="subtext">{{ log.entity_type }}</span></td><td>{{ log.action }}</td><td>{{ log.actor_type }}</td><td><span class="badge" :class="log.severity.toLowerCase()">{{ log.severity }}</span></td></tr>
              <tr v-if="auditLogs.length === 0"><td class="empty-row" colspan="5">ยังไม่มี Audit log</td></tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <div v-if="showMovieForm" class="modal-backdrop" role="presentation" @mousedown.self="closeMovieForm">
      <form class="movie-form" aria-labelledby="movie-form-title" @submit.prevent="saveMovie">
        <header><div><p>MOVIE DETAILS</p><h2 id="movie-form-title">{{ editingMovieID ? 'แก้ไขภาพยนตร์' : 'เพิ่มภาพยนตร์' }}</h2></div><button type="button" title="ปิด" @click="closeMovieForm"><X :size="20" /></button></header>
        <section class="tmdb-search" aria-labelledby="tmdb-search-title">
          <div class="tmdb-label"><strong id="tmdb-search-title">ค้นหาจาก TMDB</strong><span>เลือกหนังเพื่อเติมข้อมูลและโปสเตอร์อัตโนมัติ</span></div>
          <div class="tmdb-search-bar" role="search">
            <label class="tmdb-input">
              <Search :size="17" aria-hidden="true" />
              <input v-model="tmdbQuery" type="search" placeholder="ชื่อภาพยนตร์ เช่น Dune" aria-label="ค้นหาภาพยนตร์จาก TMDB" @keydown.enter.prevent="searchTMDB" />
            </label>
            <button type="button" :disabled="searchingTMDB || !tmdbQuery.trim()" @click="searchTMDB"><LoaderCircle v-if="searchingTMDB" class="spinner" :size="16" /><span v-else>ค้นหา</span></button>
          </div>
          <p v-if="tmdbError" class="tmdb-error" role="alert">{{ tmdbError }}</p>
          <div v-if="tmdbResults.length" class="tmdb-results">
            <button v-for="movie in tmdbResults" :key="movie.id" type="button" :disabled="selectingTMDBMovieID !== null" @click="selectTMDBMovie(movie)">
              <span class="tmdb-poster"><img v-if="movie.poster_url" :src="movie.poster_url" alt="" /><Film v-else :size="18" /></span>
              <span class="tmdb-copy"><strong>{{ movie.title }}</strong><small v-if="selectingTMDBMovieID === movie.id">กำลังดึงรายละเอียด...</small><small v-else>{{ movie.release_date?.slice(0, 4) || 'ไม่ระบุปี' }} · คะแนน {{ movie.vote_average.toFixed(1) }}</small></span>
            </button>
          </div>
        </section>
        <label>ชื่อภาพยนตร์<input v-model.trim="movieForm.title" required maxlength="200" /></label>
        <label>คำอธิบาย<textarea v-model.trim="movieForm.description" rows="4"></textarea></label>
        <div class="form-grid"><label>ความยาว (นาที)<input v-model.number="movieForm.duration_minutes" type="number" required min="1" max="1000" /></label><label>สถานะ<select v-model="movieForm.is_active"><option :value="true">เปิดแสดง</option><option :value="false">ปิดแสดง</option></select></label></div>
        <label>URL โปสเตอร์<input v-model.trim="movieForm.poster_url" type="url" placeholder="https://..." /></label>
        <footer><button class="secondary-button" type="button" @click="closeMovieForm">ยกเลิก</button><button class="primary-button" type="submit" :disabled="saving"><LoaderCircle v-if="saving" class="spinner" :size="17" />{{ saving ? 'กำลังบันทึก' : 'บันทึก' }}</button></footer>
      </form>
    </div>
  </main>
</template>

<style scoped>
.admin-page { min-height: 100vh; color: #24211f; background: #eeece8; }
.admin-layout { width: min(1240px, calc(100% - 48px)); margin: 0 auto; padding: 42px 0 70px; }
.admin-heading { display: flex; align-items: end; justify-content: space-between; gap: 24px; }
.admin-heading p, .movie-form header p { margin: 0 0 7px; color: #927027; font-size: 13px; font-weight: 800; }
.admin-heading h1 { margin: 0; font-family: Georgia, "Times New Roman", serif; font-size: 40px; font-weight: 500; letter-spacing: 0; }
.primary-button, .secondary-button { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 8px; padding: 0 15px; font-weight: 700; border-radius: 4px; cursor: pointer; }
.primary-button { color: #fff; background: #272421; border: 1px solid #272421; }
.secondary-button { color: #37322d; background: #fff; border: 1px solid #bcb6ae; }
.heading-actions { display: flex; gap: 8px; }
.heading-actions a { font-size: 15px; text-decoration: none; }
.primary-button:disabled { cursor: wait; opacity: .7; }
.metrics { display: grid; grid-template-columns: repeat(3, 1fr); margin: 30px 0; background: #fff; border: 1px solid #d6d1ca; }
.metrics > div { display: grid; grid-template-columns: auto 1fr; gap: 3px 11px; padding: 20px 22px; border-right: 1px solid #dedad4; }
.metrics > div:last-child { border-right: 0; }
.metrics svg { grid-row: 1 / 3; align-self: center; color: #9b762c; }
.metrics span { color: #777068; font-size: 14px; }
.metrics strong { font-size: 23px; }
.admin-tabs { display: flex; gap: 24px; border-bottom: 1px solid #cbc6bf; }
.admin-tabs button { position: relative; min-height: 46px; padding: 0 2px; color: #716b64; font-size: 15px; font-weight: 700; background: transparent; border: 0; cursor: pointer; }
.admin-tabs button.active { color: #25221f; }
.admin-tabs button.active::after { position: absolute; right: 0; bottom: -1px; left: 0; height: 3px; content: ""; background: #a27b2d; }
.admin-content { padding-top: 22px; }
.toolbar { display: flex; width: min(420px, 100%); min-height: 42px; align-items: center; margin-bottom: 16px; background: #fff; border: 1px solid #cbc6bf; border-radius: 4px; }
.toolbar svg { margin-left: 12px; color: #777068; }
.toolbar input { min-width: 0; flex: 1; height: 40px; padding: 0 10px; border: 0; outline: 0; }
.toolbar button { height: 34px; margin-right: 4px; padding: 0 14px; color: #fff; background: #37332f; border: 0; border-radius: 3px; cursor: pointer; }
.booking-filter { width: 100%; flex-wrap: wrap; padding: 10px; gap: 10px; }
.booking-filter label, .audit-filter label { display: grid; gap: 4px; color: #706a63; font-size: 12px; font-weight: 700; }
.booking-filter select, .booking-filter input, .audit-filter select { min-width: 130px; height: 36px; padding: 0 8px; background: #fff; border: 1px solid #d2cdc6; border-radius: 3px; }
.booking-filter label:nth-child(3) select { width: min(260px, 70vw); }
.booking-filter button { display: inline-flex; align-self: end; align-items: center; gap: 5px; height: 36px; }
.booking-filter button svg { margin-left: 0; color: currentColor; }
.booking-filter .clear-filter { color: #37332f; background: #fff; border: 1px solid #c9c4bd; }
.audit-filter { width: fit-content; padding: 8px 10px; }
.table-wrap { overflow-x: auto; background: #fff; border: 1px solid #d6d1ca; }
table { width: 100%; min-width: 850px; border-collapse: collapse; }
th { padding: 12px 15px; color: #6f6962; font-size: 13px; text-align: left; background: #f6f5f2; border-bottom: 1px solid #d6d1ca; }
td { padding: 14px 15px; font-size: 15px; border-bottom: 1px solid #ebe8e3; vertical-align: middle; }
tr:last-child td { border-bottom: 0; }
.movie-cell { display: flex; min-width: 270px; align-items: center; gap: 12px; }
.movie-cell > div:last-child { display: grid; gap: 4px; }
.movie-cell span, .subtext { display: block; max-width: 330px; overflow: hidden; color: #807a73; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.poster { display: grid; width: 38px; height: 52px; flex: 0 0 auto; place-items: center; overflow: hidden; color: #8c857d; background: #e9e6e1; }
.poster img { width: 100%; height: 100%; object-fit: cover; }
.status-toggle, .badge { display: inline-flex; min-height: 25px; align-items: center; padding: 0 8px; color: #236139; font-size: 13px; font-weight: 750; background: #e7f4eb; border: 1px solid #b8d8c1; border-radius: 3px; }
.status-toggle { cursor: pointer; }
.status-toggle.inactive, .badge.cancelled, .badge.error { color: #962f29; background: #fbe9e7; border-color: #e6b6b2; }
.badge.warning { color: #7c5b17; background: #fbf1d8; border-color: #e5cf98; }
.row-actions { display: flex; justify-content: end; gap: 4px; }
.row-actions button, .movie-form header button { display: grid; width: 34px; height: 34px; place-items: center; color: #625c55; background: transparent; border: 1px solid transparent; border-radius: 3px; cursor: pointer; }
.row-actions button:hover { background: #f0eeea; border-color: #d5d0c9; }
.row-actions .danger { color: #a43a32; }
.code { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
.empty-row { height: 130px; color: #79736c; text-align: center; }
.loading-state { display: flex; min-height: 260px; align-items: center; justify-content: center; gap: 10px; color: #716b64; }
.spinner { animation: spin .85s linear infinite; }
.admin-error { margin: 16px 0 0; padding: 11px 13px; color: #922e27; background: #fceae8; border-left: 3px solid #b84138; }
.modal-backdrop { position: fixed; z-index: 50; inset: 0; display: grid; place-items: center; padding: 20px; background: rgba(20, 18, 16, .62); }
.movie-form { width: min(560px, 100%); max-height: calc(100vh - 40px); overflow-y: auto; padding: 26px; background: #fff; border-radius: 6px; box-shadow: 0 24px 70px rgba(0,0,0,.3); }
.movie-form header { display: flex; align-items: start; justify-content: space-between; margin-bottom: 22px; }
.movie-form h2 { margin: 0; font-family: Georgia, "Times New Roman", serif; font-size: 30px; font-weight: 500; letter-spacing: 0; }
.tmdb-search { margin-bottom: 22px; padding: 15px; background: #f4f2ee; border: 1px solid #d8d2ca; }
.tmdb-label { display: grid; gap: 3px; margin-bottom: 10px; }
.tmdb-label strong { font-size: 15px; }
.tmdb-label span { color: #756f68; font-size: 13px; }
.tmdb-search-bar { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; background: #fff; border: 1px solid #c8c2ba; border-radius: 3px; }
.tmdb-input { position: relative; display: block; min-width: 0; }
.tmdb-input svg { position: absolute; top: 50%; left: 11px; margin: 0; color: #777068; pointer-events: none; transform: translateY(-50%); }
.movie-form .tmdb-input input { min-width: 0; width: 100%; height: 39px; min-height: 39px; padding: 0 9px 0 38px; border: 0; outline: 0; }
.tmdb-search-bar button { display: grid; min-width: 72px; height: 33px; margin-right: 3px; place-items: center; color: #fff; background: #34302c; border: 0; border-radius: 3px; cursor: pointer; }
.tmdb-search-bar button:disabled { cursor: not-allowed; opacity: .55; }
.tmdb-results { display: grid; grid-template-columns: 1fr 1fr; gap: 5px; max-height: 230px; margin-top: 10px; overflow-y: auto; }
.tmdb-results > button { display: flex; min-width: 0; align-items: center; gap: 9px; padding: 6px; color: #292622; text-align: left; background: #fff; border: 1px solid #ddd8d1; border-radius: 3px; cursor: pointer; }
.tmdb-results > button:hover { border-color: #9c7a36; }
.tmdb-results > button:disabled { cursor: wait; opacity: .65; }
.tmdb-poster { display: grid; width: 34px; height: 48px; flex: 0 0 auto; place-items: center; overflow: hidden; color: #8b857e; background: #e7e3de; }
.tmdb-poster img { width: 100%; height: 100%; object-fit: cover; }
.tmdb-copy { display: grid; min-width: 0; gap: 4px; }
.tmdb-copy strong { overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.tmdb-copy small { color: #777068; font-size: 12px; }
.tmdb-error { margin: 8px 0 0; color: #972f28; font-size: 13px; }
.movie-form > label, .form-grid label { display: grid; gap: 7px; margin-bottom: 16px; color: #4e4943; font-size: 14px; font-weight: 700; }
.movie-form input, .movie-form textarea, .movie-form select { width: 100%; min-height: 42px; padding: 9px 11px; color: #282522; background: #fff; border: 1px solid #c9c4bd; border-radius: 3px; }
.movie-form textarea { resize: vertical; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.movie-form footer { display: flex; justify-content: end; gap: 8px; padding-top: 8px; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0,0,0,0); }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 700px) {
  .admin-layout { width: calc(100% - 28px); padding-top: 30px; }
  .admin-heading { align-items: start; }
  .heading-actions { flex-direction: column; }
  .admin-heading h1 { font-size: 33px; }
  .metrics { grid-template-columns: 1fr; }
  .metrics > div { border-right: 0; border-bottom: 1px solid #dedad4; }
  .metrics > div:last-child { border-bottom: 0; }
  .form-grid { grid-template-columns: 1fr; gap: 0; }
  .tmdb-results { grid-template-columns: 1fr; }
}
@media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
</style>

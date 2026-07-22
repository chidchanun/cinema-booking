<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { CalendarDays, Clock, Film, LoaderCircle, MapPin } from '@lucide/vue'

import { ApiError } from '@/services/api'
import { getMovie, listMovieShowtimes } from '@/services/movies'
import type { Movie, Showtime } from '@/types/movie'
import SiteHeader from '@/components/SiteHeader.vue'

const props = defineProps<{ movieId: string }>()

const movie = ref<Movie | null>(null)
const showtimes = ref<Showtime[]>([])
const loading = ref(true)
const errorMessage = ref('')
const posterFailed = ref(false)

const groupedShowtimes = computed(() => {
  const groups = new Map<string, Showtime[]>()
  for (const showtime of showtimes.value) {
    const key = new Intl.DateTimeFormat('th-TH', {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    }).format(new Date(showtime.start_time))
    groups.set(key, [...(groups.get(key) ?? []), showtime])
  }
  return [...groups.entries()]
})

async function loadMovie() {
  loading.value = true
  errorMessage.value = ''
  posterFailed.value = false

  try {
    const [movieResponse, showtimeResponse] = await Promise.all([
      getMovie(props.movieId),
      listMovieShowtimes(props.movieId),
    ])
    movie.value = movieResponse
    showtimes.value = showtimeResponse.data
  } catch (error) {
    errorMessage.value =
      error instanceof ApiError ? error.message : 'ไม่สามารถโหลดข้อมูลภาพยนตร์ได้'
  } finally {
    loading.value = false
  }
}

function formatTime(value: string) {
  const time = new Intl.DateTimeFormat('th-TH', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value))
  return `${time} น.`
}

function formatPrice(showtime: Showtime) {
  return new Intl.NumberFormat('th-TH', {
    style: 'currency',
    currency: showtime.currency,
    maximumFractionDigits: 0,
  }).format(showtime.price)
}

onMounted(loadMovie)
watch(() => props.movieId, loadMovie)
</script>

<template>
  <main class="movie-detail-page">
    <SiteHeader back-to="/" back-label="ภาพยนตร์ทั้งหมด" />

    <div v-if="loading" class="detail-status">
      <LoaderCircle class="detail-spinner" :size="30" />
      <span>กำลังโหลดข้อมูลภาพยนตร์</span>
    </div>

    <div v-else-if="errorMessage || !movie" class="detail-status error-detail">
      <Film :size="38" stroke-width="1.4" />
      <h1>ไม่พบข้อมูลภาพยนตร์</h1>
      <p>{{ errorMessage }}</p>
      <RouterLink to="/">กลับไปหน้าภาพยนตร์</RouterLink>
    </div>

    <template v-else>
      <section class="movie-overview">
        <div class="detail-poster">
          <img
            v-if="movie.poster_url && !posterFailed"
            :src="movie.poster_url"
            :alt="`โปสเตอร์ ${movie.title}`"
            @error="posterFailed = true"
          />
          <div v-else class="detail-poster-fallback">
            <Film :size="52" stroke-width="1.3" />
            <span>{{ movie.title }}</span>
          </div>
        </div>

        <div class="overview-copy">
          <p class="detail-label">NOW SHOWING</p>
          <h1>{{ movie.title }}</h1>
          <div class="movie-metadata">
            <span><Clock :size="17" /> {{ movie.duration_minutes }} นาที</span>
            <span :class="['status-badge', { inactive: !movie.is_active }]">
              {{ movie.is_active ? 'กำลังฉาย' : 'ปิดการขาย' }}
            </span>
          </div>
          <div class="description-block">
            <h2>เรื่องย่อ</h2>
            <p>{{ movie.description || 'ยังไม่มีเรื่องย่อสำหรับภาพยนตร์เรื่องนี้' }}</p>
          </div>
        </div>
      </section>

      <section class="showtime-section">
        <div class="showtime-heading">
          <div>
            <p class="detail-label">SHOWTIMES</p>
            <h2>เลือกรอบฉาย</h2>
          </div>
          <p>รอบฉายในช่วง 30 วันข้างหน้า</p>
        </div>

        <div v-if="groupedShowtimes.length === 0" class="empty-showtimes">
          <CalendarDays :size="32" stroke-width="1.5" />
          <h3>ยังไม่มีรอบฉาย</h3>
          <p>โปรดกลับมาตรวจสอบอีกครั้งในภายหลัง</p>
        </div>

        <div v-else class="showtime-groups">
          <section v-for="[date, items] in groupedShowtimes" :key="date" class="showtime-day">
            <h3><CalendarDays :size="18" /> {{ date }}</h3>
            <div class="showtime-list">
              <RouterLink v-for="showtime in items" :key="showtime.id" class="showtime-item" :to="`/booking/${showtime.id}`">
                <strong>{{ formatTime(showtime.start_time) }}</strong>
                <span class="hall"><MapPin :size="15" /> {{ showtime.hall_name }}</span>
                <span>{{ formatPrice(showtime) }}</span>
                <span class="seat-count">{{ showtime.total_seats }} ที่นั่ง</span>
                <span class="booking-label">เลือกที่นั่ง</span>
              </RouterLink>
            </div>
          </section>
        </div>
      </section>
    </template>
  </main>
</template>

<style scoped>
.movie-detail-page {
  min-height: 100vh;
  color: #24211f;
  background: #f4f3f0;
}

.movie-overview {
  display: grid;
  grid-template-columns: minmax(230px, 330px) minmax(0, 1fr);
  gap: 64px;
  width: min(1120px, calc(100% - 48px));
  margin: 0 auto;
  padding: 56px 0 64px;
}

.detail-poster {
  overflow: hidden;
  aspect-ratio: 2 / 3;
  background: #282421;
  border: 1px solid #c6c0b8;
  border-radius: 6px;
  box-shadow: 0 18px 45px rgba(43, 37, 32, 0.18);
}

.detail-poster img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-poster-fallback {
  display: flex;
  height: 100%;
  padding: 32px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 20px;
  color: #e0c778;
  text-align: center;
  background-color: #2b2723;
  background-image: url("@/assets/cinema-login.png");
  background-position: center;
  background-size: cover;
  background-blend-mode: multiply;
}

.detail-poster-fallback span {
  overflow-wrap: anywhere;
  color: #ffffff;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 28px;
  line-height: 1.25;
}

.overview-copy {
  align-self: center;
}

.detail-label {
  margin: 0 0 12px;
  color: #916d22;
  font-size: 13px;
  font-weight: 800;
}

.overview-copy h1 {
  max-width: 680px;
  margin: 0;
  overflow-wrap: anywhere;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 54px;
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: 0;
}

.movie-metadata {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-top: 22px;
  color: #67615b;
  font-size: 16px;
}

.movie-metadata > span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.status-badge {
  padding: 5px 9px;
  color: #285c3a;
  font-size: 14px;
  font-weight: 750;
  background: #e4efe7;
  border-radius: 3px;
}

.status-badge.inactive {
  color: #7a312c;
  background: #f2e3e1;
}

.description-block {
  max-width: 710px;
  margin-top: 42px;
  padding-top: 28px;
  border-top: 1px solid #cbc6bf;
}

.description-block h2 {
  margin: 0 0 12px;
  font-size: 17px;
  letter-spacing: 0;
}

.description-block p {
  margin: 0;
  color: #5f5953;
  font-size: 17px;
  line-height: 1.85;
  white-space: pre-line;
}

.showtime-section {
  padding: 52px max(24px, calc((100% - 1120px) / 2)) 80px;
  background: #ffffff;
  border-top: 1px solid #d5d1ca;
}

.showtime-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid #d9d5cf;
}

.showtime-heading h2 {
  margin: 0;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 34px;
  font-weight: 500;
  letter-spacing: 0;
}

.showtime-heading > p {
  margin: 0;
  color: #77716a;
  font-size: 15px;
}

.showtime-day {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 28px;
  padding: 28px 0;
  border-bottom: 1px solid #e2dfda;
}

.showtime-day h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 16px;
  letter-spacing: 0;
}

.showtime-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.showtime-item {
  display: grid;
  grid-template-columns: 84px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  min-height: 68px;
  padding: 12px 14px;
  background: #f7f6f3;
  border: 1px solid #dcd8d2;
  border-radius: 5px;
  color: inherit;
  text-decoration: none;
  transition: border-color 150ms ease, background 150ms ease;
}
.showtime-item:hover { background: #fff; border-color: #a9853c; }

.showtime-item strong {
  font-size: 20px;
  white-space: nowrap;
}

.showtime-item > span {
  color: #5f5953;
  font-size: 15px;
}

.showtime-item .hall {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 5px;
  overflow-wrap: anywhere;
}

.showtime-item .seat-count {
  grid-column: 2;
  color: #888179;
  font-size: 13px;
}
.showtime-item .booking-label { grid-column: 3; grid-row: 2; color: #8a6720; font-weight: 750; white-space: nowrap; }

.empty-showtimes,
.detail-status {
  display: flex;
  min-height: 300px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 11px;
  color: #746e67;
  text-align: center;
}

.empty-showtimes h3,
.empty-showtimes p,
.detail-status h1,
.detail-status p {
  margin: 0;
}

.detail-status {
  min-height: calc(100vh - 72px);
}

.detail-status a {
  margin-top: 8px;
  color: #74591f;
  font-weight: 700;
}

.error-detail {
  color: #7f302b;
}

.detail-spinner {
  animation: spin 0.85s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 820px) {
  .movie-overview {
    grid-template-columns: 220px minmax(0, 1fr);
    gap: 32px;
  }

  .overview-copy h1 {
    font-size: 42px;
  }

  .showtime-day {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 620px) {
  .movie-overview {
    grid-template-columns: 1fr;
    align-items: start;
    gap: 20px;
    width: calc(100% - 28px);
    padding: 32px 0 42px;
  }

  .detail-poster {
    width: 160px;
    border-radius: 4px;
  }

  .detail-poster-fallback {
    padding: 12px;
  }

  .detail-poster-fallback svg,
  .detail-poster-fallback span {
    display: none;
  }

  .overview-copy {
    align-self: start;
  }

  .overview-copy h1 {
    font-size: 33px;
  }

  .movie-metadata {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
    margin-top: 16px;
  }

  .showtime-section {
    padding: 38px 14px 60px;
  }

  .showtime-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .showtime-list {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .detail-spinner {
    animation: none;
  }
}
</style>

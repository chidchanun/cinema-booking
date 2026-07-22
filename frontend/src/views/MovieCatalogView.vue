<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Clock, Film, LoaderCircle, Search } from '@lucide/vue'

import { ApiError } from '@/services/api'
import { listMovies } from '@/services/movies'
import type { Movie } from '@/types/movie'
import SiteHeader from '@/components/SiteHeader.vue'

const movies = ref<Movie[]>([])
const search = ref('')
const loading = ref(true)
const errorMessage = ref('')
const failedPosters = ref(new Set<string>())

async function loadMovies() {
  loading.value = true
  errorMessage.value = ''

  try {
    const response = await listMovies(search.value)
    movies.value = response.data
  } catch (error) {
    errorMessage.value =
      error instanceof ApiError ? error.message : 'ไม่สามารถโหลดรายการภาพยนตร์ได้'
  } finally {
    loading.value = false
  }
}

function markPosterFailed(movieID: string) {
  failedPosters.value = new Set(failedPosters.value).add(movieID)
}

onMounted(loadMovies)
</script>

<template>
  <main class="catalog-page">
    <SiteHeader section="ภาพยนตร์" />

    <section class="catalog-heading">
      <div>
        <p class="section-label">NOW SHOWING</p>
        <h1>ภาพยนตร์ที่กำลังฉาย</h1>
        <p>เลือกเรื่องที่สนใจเพื่อดูรายละเอียดและรอบฉาย</p>
      </div>

      <form class="movie-search" role="search" @submit.prevent="loadMovies">
        <Search :size="19" aria-hidden="true" />
        <input v-model="search" type="search" placeholder="ค้นหาชื่อภาพยนตร์" aria-label="ค้นหาภาพยนตร์" />
        <button type="submit">ค้นหา</button>
      </form>
    </section>

    <section class="catalog-content" aria-live="polite">
      <div v-if="loading" class="catalog-status">
        <LoaderCircle class="catalog-spinner" :size="28" />
        <span>กำลังโหลดภาพยนตร์</span>
      </div>

      <div v-else-if="errorMessage" class="catalog-status error-status">
        <p>{{ errorMessage }}</p>
        <button type="button" @click="loadMovies">ลองอีกครั้ง</button>
      </div>

      <div v-else-if="movies.length === 0" class="catalog-status">
        <Film :size="34" stroke-width="1.5" />
        <h2>ยังไม่พบภาพยนตร์</h2>
        <p>ลองเปลี่ยนคำค้นหา หรือตรวจสอบว่ามีภาพยนตร์เปิดขายแล้ว</p>
      </div>

      <div v-else class="movie-grid">
        <RouterLink
          v-for="movie in movies"
          :key="movie.id"
          class="movie-card"
          :to="`/movies/${movie.id}`"
        >
          <div class="poster-frame">
            <img
              v-if="movie.poster_url && !failedPosters.has(movie.id)"
              :src="movie.poster_url"
              :alt="`โปสเตอร์ ${movie.title}`"
              loading="lazy"
              @error="markPosterFailed(movie.id)"
            />
            <div v-else class="poster-fallback">
              <Film :size="36" stroke-width="1.4" />
              <span>{{ movie.title }}</span>
            </div>
          </div>
          <div class="movie-card-copy">
            <h2>{{ movie.title }}</h2>
            <p class="duration"><Clock :size="15" /> {{ movie.duration_minutes }} นาที</p>
            <p class="description">{{ movie.description }}</p>
            <span class="detail-link">ดูรายละเอียด</span>
          </div>
        </RouterLink>
      </div>
    </section>
  </main>
</template>

<style scoped>
.catalog-page {
  min-height: 100vh;
  color: #24211f;
  background: #f4f3f0;
}

.catalog-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 40px;
  width: min(1180px, calc(100% - 48px));
  margin: 0 auto;
  padding: 56px 0 34px;
  border-bottom: 1px solid #cfcbc5;
}

.section-label {
  margin: 0 0 10px;
  color: #936f25;
  font-size: 13px;
  font-weight: 800;
}

.catalog-heading h1 {
  margin: 0;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 42px;
  font-weight: 500;
  line-height: 1.15;
  letter-spacing: 0;
}

.catalog-heading > div > p:last-child {
  margin: 12px 0 0;
  color: #6f6963;
  font-size: 16px;
}

.movie-search {
  display: grid;
  grid-template-columns: auto minmax(150px, 260px) auto;
  align-items: center;
  min-height: 44px;
  background: #ffffff;
  border: 1px solid #c9c4bd;
  border-radius: 4px;
}

.movie-search svg {
  margin-left: 13px;
  color: #79736d;
}

.movie-search input {
  min-width: 0;
  height: 42px;
  padding: 0 10px;
  color: #282522;
  background: transparent;
  border: 0;
  outline: 0;
}

.movie-search button,
.catalog-status button {
  min-height: 36px;
  margin-right: 4px;
  padding: 0 16px;
  color: #ffffff;
  font-weight: 700;
  background: #2c2926;
  border: 0;
  border-radius: 3px;
  cursor: pointer;
}

.catalog-content {
  width: min(1180px, calc(100% - 48px));
  margin: 0 auto;
  padding: 36px 0 64px;
}

.movie-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 28px 20px;
}

.movie-card {
  min-width: 0;
  overflow: hidden;
  color: inherit;
  text-decoration: none;
  background: #ffffff;
  border: 1px solid #d8d4ce;
  border-radius: 6px;
  transition:
    transform 160ms ease,
    box-shadow 160ms ease;
}

.movie-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 14px 34px rgba(41, 36, 31, 0.12);
}

.poster-frame {
  aspect-ratio: 2 / 3;
  overflow: hidden;
  background: #24211f;
}

.poster-frame img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.poster-fallback {
  display: flex;
  height: 100%;
  padding: 24px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  color: #d9c387;
  text-align: center;
  background-color: #27231f;
  background-image: url("@/assets/cinema-login.png");
  background-position: center;
  background-size: cover;
  background-blend-mode: multiply;
}

.poster-fallback span {
  overflow-wrap: anywhere;
  color: #ffffff;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 22px;
  line-height: 1.3;
}

.movie-card-copy {
  padding: 18px;
}

.movie-card h2 {
  min-height: 48px;
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 19px;
  line-height: 1.4;
  letter-spacing: 0;
}

.duration {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 10px 0 0;
  color: #756e67;
  font-size: 15px;
}

.description {
  display: -webkit-box;
  min-height: 60px;
  margin: 14px 0 16px;
  overflow: hidden;
  color: #6a645e;
  font-size: 15px;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.detail-link {
  color: #795b1f;
  font-size: 15px;
  font-weight: 750;
}

.catalog-status {
  display: flex;
  min-height: 320px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #716b65;
  text-align: center;
}

.catalog-status h2,
.catalog-status p {
  margin: 0;
}

.catalog-status button {
  margin: 8px 0 0;
  padding: 0 20px;
}

.error-status {
  color: #8e2d27;
}

.catalog-spinner {
  animation: spin 0.85s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 960px) {
  .movie-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .catalog-heading {
    align-items: stretch;
    flex-direction: column;
    padding-top: 38px;
  }

  .movie-search {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .movie-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 440px) {
  .catalog-heading,
  .catalog-content {
    width: calc(100% - 28px);
  }

  .catalog-heading h1 {
    font-size: 36px;
  }

  .movie-grid {
    grid-template-columns: 1fr;
  }

  .movie-card {
    display: grid;
    grid-template-columns: 120px minmax(0, 1fr);
  }

  .poster-frame {
    min-height: 210px;
    aspect-ratio: auto;
  }

  .movie-card h2,
  .description {
    min-height: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .movie-card {
    transition: none;
  }

  .catalog-spinner {
    animation: none;
  }
}
</style>

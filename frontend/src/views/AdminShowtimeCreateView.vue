<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Building2, CalendarClock, Check, CircleAlert, LoaderCircle, Save, ScreenShare } from '@lucide/vue'
import { useRouter } from 'vue-router'

import SiteHeader from '@/components/SiteHeader.vue'
import { ApiError } from '@/services/api'
import { checkHallAvailability, createShowtime, listAdminMovies, listHalls } from '@/services/admin'
import type { Movie, Showtime } from '@/types/movie'

const router = useRouter()
const movies = ref<Movie[]>([])
const loadingMovies = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const createdShowtime = ref<Showtime | null>(null)
const halls = ref<string[]>([])
const hallChoice = ref('')
const checkingAvailability = ref(false)
const hallAvailable = ref<boolean | null>(null)
let availabilityTimer: ReturnType<typeof setTimeout> | undefined
let availabilityRequest = 0

function defaultStartTime(): string {
  const date = new Date(Date.now() + 60 * 60 * 1000)
  date.setMinutes(Math.ceil(date.getMinutes() / 15) * 15, 0, 0)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

const form = reactive({
  movie_id: '',
  hall_name: '',
  start_time: defaultStartTime(),
  price: 250,
  currency: 'THB',
  seat_rows: 8,
  seats_per_row: 12,
})

const activeMovies = computed(() => movies.value.filter((movie) => movie.is_active))
const selectedMovie = computed(() => movies.value.find((movie) => movie.id === form.movie_id))
const addingNewHall = computed(() => hallChoice.value === '__new__')
const totalSeats = computed(() => form.seat_rows * form.seats_per_row)
const estimatedEnd = computed(() => {
  if (!form.start_time || !selectedMovie.value) return ''
  const end = new Date(form.start_time)
  end.setMinutes(end.getMinutes() + selectedMovie.value.duration_minutes + 15)
  return `${new Intl.DateTimeFormat('th-TH', { dateStyle: 'medium', timeStyle: 'short' }).format(end)} น.`
})
const minimumStart = computed(() => {
  const date = new Date(Date.now() + 5 * 60 * 1000)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
})

async function loadMovies(): Promise<void> {
  try {
    const [movieResponse, hallResponse] = await Promise.all([listAdminMovies(), listHalls()])
    movies.value = movieResponse.data
    halls.value = hallResponse.data
    hallChoice.value = halls.value.length > 0 ? '' : '__new__'
    if (activeMovies.value.length === 1) form.movie_id = activeMovies.value[0]?.id ?? ''
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'ไม่สามารถโหลดรายการภาพยนตร์ได้'
  } finally {
    loadingMovies.value = false
  }
}

watch(hallChoice, (choice) => {
  form.hall_name = choice === '__new__' ? '' : choice
})

async function verifyHallAvailability(): Promise<void> {
  if (!form.movie_id || !form.hall_name.trim() || !form.start_time) {
    hallAvailable.value = null
    return
  }

  const requestID = ++availabilityRequest
  checkingAvailability.value = true
  try {
    const result = await checkHallAvailability({
      movieID: form.movie_id,
      hallName: form.hall_name,
      startTime: new Date(form.start_time).toISOString(),
    })
    if (requestID !== availabilityRequest) return
    hallAvailable.value = result.available
  } catch {
    if (requestID === availabilityRequest) hallAvailable.value = null
  } finally {
    if (requestID === availabilityRequest) checkingAvailability.value = false
  }
}

watch(
  () => [form.movie_id, form.hall_name, form.start_time],
  () => {
    if (availabilityTimer) clearTimeout(availabilityTimer)
    availabilityRequest++
    hallAvailable.value = null
    createdShowtime.value = null
    availabilityTimer = setTimeout(() => void verifyHallAvailability(), 400)
  },
)

async function submitShowtime(): Promise<void> {
  if (saving.value || totalSeats.value > 1000) return
  saving.value = true
  errorMessage.value = ''
  createdShowtime.value = null
  try {
    createdShowtime.value = await createShowtime({
      ...form,
      hall_name: form.hall_name.trim(),
      start_time: new Date(form.start_time).toISOString(),
      currency: form.currency.trim().toUpperCase(),
    })
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'ไม่สามารถสร้างรอบฉายได้'
  } finally {
    saving.value = false
  }
}

onMounted(loadMovies)
onBeforeUnmount(() => {
  if (availabilityTimer) clearTimeout(availabilityTimer)
  availabilityRequest++
})
</script>

<template>
  <main class="showtime-page">
    <SiteHeader back-to="/admin" back-label="กลับหน้าจัดการระบบ" />
    <div class="showtime-layout">
      <header class="page-heading">
        <div><p>SHOWTIME SCHEDULING</p><h1>เพิ่มรอบฉาย</h1><span>กำหนดเวลา โรงฉาย ราคา และรูปแบบที่นั่ง</span></div>
      </header>

      <p v-if="errorMessage" class="form-message error" role="alert">{{ errorMessage }}</p>
      <div v-if="createdShowtime" class="form-message success" role="status">
        <Check :size="19" /><div><strong>สร้างรอบฉายสำเร็จ</strong><span>{{ selectedMovie?.title }} · {{ createdShowtime.hall_name }}</span></div>
        <button type="button" @click="router.push(`/movies/${createdShowtime.movie_id}`)">ดูหน้าภาพยนตร์</button>
      </div>

      <form class="showtime-form" @submit.prevent="submitShowtime">
        <section class="form-section">
          <div class="section-heading"><CalendarClock :size="20" /><div><h2>ข้อมูลรอบฉาย</h2><p>เพิ่มรอบในช่วงเวลาก่อนหรือหลังรอบเดิมได้ หากเวลาฉายรวมเวลาเตรียม Hall 15 นาทีไม่ทับกัน</p></div></div>
          <div class="fields two-columns">
            <label class="wide">ภาพยนตร์<select v-model="form.movie_id" required :disabled="loadingMovies"><option value="" disabled>{{ loadingMovies ? 'กำลังโหลดภาพยนตร์' : 'เลือกภาพยนตร์' }}</option><option v-for="movie in activeMovies" :key="movie.id" :value="movie.id">{{ movie.title }} ({{ movie.duration_minutes }} นาที)</option></select></label>
            <label>Hall<span class="hall-input"><Building2 :size="17" /><select v-model="hallChoice" required><option value="" disabled>เลือก Hall</option><option v-for="hall in halls" :key="hall" :value="hall">{{ hall }}</option><option value="__new__">+ เพิ่ม Hall ใหม่</option></select></span><small>{{ halls.length ? `พบ Hall เดิม ${halls.length} รายการ` : 'ยังไม่มี Hall ในระบบ' }}</small></label>
            <label v-if="addingNewHall">ชื่อ Hall ใหม่<input v-model.trim="form.hall_name" required maxlength="100" placeholder="เช่น Hall 2" /><small>ชื่อ Hall นี้จะปรากฏในรายการครั้งถัดไป</small></label>
            <label>วันและเวลาเริ่ม<input v-model="form.start_time" type="datetime-local" required :min="minimumStart" /></label>
          </div>
          <div v-if="selectedMovie && estimatedEnd" class="time-summary"><span>เวลาสิ้นสุดโดยประมาณ</span><strong>{{ estimatedEnd }}</strong><small>รวมเวลาเตรียมโรงแล้ว</small></div>
          <div v-if="checkingAvailability" class="availability checking"><LoaderCircle class="spinner" :size="17" />กำลังตรวจสอบเวลาของ Hall</div>
          <div v-else-if="hallAvailable === true" class="availability available"><Check :size="17" />Hall นี้ว่างในช่วงเวลาที่เลือก</div>
          <div v-else-if="hallAvailable === false" class="availability conflict" role="alert"><CircleAlert :size="17" /><span><strong>ช่วงเวลานี้มีรอบฉายอื่นอยู่แล้ว</strong> เลือกเวลาเริ่มหลังจากรอบเดิมและเวลาเตรียมโรงสิ้นสุด</span></div>
        </section>

        <section class="form-section">
          <div class="section-heading"><ScreenShare :size="20" /><div><h2>ราคาและผังที่นั่ง</h2><p>รหัสที่นั่งจะสร้างอัตโนมัติตั้งแต่แถว A</p></div></div>
          <div class="fields four-columns">
            <label>ราคา<input v-model.number="form.price" type="number" required min="0" step="1" /></label>
            <label>สกุลเงิน<input v-model.trim="form.currency" required minlength="3" maxlength="3" disabled/></label>
            <label>จำนวนแถว<input v-model.number="form.seat_rows" type="number" required min="1" max="26" /></label>
            <label>ที่นั่งต่อแถว<input v-model.number="form.seats_per_row" type="number" required min="1" max="50" /></label>
          </div>
          <div class="seat-preview" :class="{ invalid: totalSeats > 1000 }"><span>จำนวนที่นั่งทั้งหมด</span><strong>{{ totalSeats }}</strong><small v-if="totalSeats > 1000">จำนวนที่นั่งต้องไม่เกิน 1,000 ที่</small><small v-else>แถว A–{{ String.fromCharCode(64 + form.seat_rows) }} · หมายเลข 1–{{ form.seats_per_row }}</small></div>
        </section>

        <footer class="form-actions"><RouterLink to="/admin">ยกเลิก</RouterLink><button type="submit" :disabled="saving || loadingMovies || totalSeats > 1000 || hallAvailable === false || checkingAvailability"><LoaderCircle v-if="saving" class="spinner" :size="18" /><Save v-else :size="18" />{{ saving ? 'กำลังสร้างรอบฉาย' : 'สร้างรอบฉาย' }}</button></footer>
      </form>
    </div>
  </main>
</template>

<style scoped>
.showtime-page { min-height: 100vh; color: #292521; background: #efede9; }
.showtime-layout { width: min(980px, calc(100% - 48px)); margin: 0 auto; padding: 46px 0 70px; }
.page-heading { padding-bottom: 26px; border-bottom: 1px solid #cec9c2; }
.page-heading p { margin: 0 0 8px; color: #927027; font-size: 13px; font-weight: 800; }
.page-heading h1 { margin: 0; font-family: Georgia, "Times New Roman", serif; font-size: 41px; font-weight: 500; letter-spacing: 0; }
.page-heading span { display: block; margin-top: 9px; color: #746e67; font-size: 15px; }
.showtime-form { margin-top: 24px; background: #fff; border: 1px solid #d4cfc8; }
.form-section { padding: 27px 30px; border-bottom: 1px solid #ded9d2; }
.section-heading { display: flex; gap: 11px; margin-bottom: 22px; }
.section-heading svg { color: #957229; }
.section-heading h2, .section-heading p { margin: 0; }
.section-heading h2 { font-size: 18px; }
.section-heading p { margin-top: 4px; color: #817a72; font-size: 13px; }
.fields { display: grid; gap: 17px; align-items: start; }
.two-columns { grid-template-columns: 1fr 1fr; }
.four-columns { grid-template-columns: repeat(4, 1fr); }
.wide { grid-column: 1 / -1; }
.fields label { display: flex; min-width: 0; flex-direction: column; align-self: start; gap: 7px; color: #544f49; font-size: 14px; font-weight: 700; }
.fields input, .fields select { width: 100%; min-width: 0; height: 43px; padding: 0 11px; color: #2d2925; background: #fff; border: 1px solid #c7c1b9; border-radius: 3px; }
.fields label > small { color: #8a837b; font-size: 12px; font-weight: 500; }
.hall-input { position: relative; display: block; }
.hall-input svg { position: absolute; top: 50%; left: 11px; color: #777068; pointer-events: none; transform: translateY(-50%); }
.fields .hall-input select { padding-left: 38px; }
.time-summary, .seat-preview { display: grid; grid-template-columns: 1fr auto; gap: 4px 18px; margin-top: 18px; padding: 13px 15px; background: #f5f3ef; border-left: 3px solid #a37c30; }
.time-summary span, .seat-preview span { color: #6e6861; font-size: 14px; }
.time-summary strong, .seat-preview strong { grid-row: 1 / 3; grid-column: 2; align-self: center; }
.time-summary small, .seat-preview small { color: #8a837b; font-size: 12px; }
.seat-preview.invalid { color: #9b302a; background: #fcebea; border-left-color: #b23a32; }
.availability { display: flex; min-height: 40px; align-items: center; gap: 8px; margin-top: 10px; padding: 9px 12px; font-size: 13px; border-left: 3px solid; }
.availability.checking { color: #6f6962; background: #f3f1ed; border-color: #999188; }
.availability.available { color: #245f39; background: #e8f4eb; border-color: #3e8657; }
.availability.conflict { color: #942f28; background: #fbe9e7; border-color: #b43c34; }
.availability.conflict span { display: grid; gap: 2px; }
.form-actions { display: flex; justify-content: end; gap: 9px; padding: 20px 30px; background: #f7f5f2; }
.form-actions a, .form-actions button { display: inline-flex; min-height: 41px; align-items: center; justify-content: center; gap: 8px; padding: 0 16px; font-size: 15px; font-weight: 700; text-decoration: none; border-radius: 4px; }
.form-actions a { color: #45403b; background: #fff; border: 1px solid #beb8b0; }
.form-actions button { color: #fff; background: #292622; border: 1px solid #292622; cursor: pointer; }
.form-actions button:disabled { cursor: not-allowed; opacity: .6; }
.form-message { display: flex; align-items: center; gap: 10px; margin: 18px 0 -6px; padding: 13px 15px; font-size: 14px; }
.form-message.error { color: #922e27; background: #fbe9e7; border-left: 3px solid #b33b33; }
.form-message.success { color: #235d37; background: #e9f4ec; border-left: 3px solid #388453; }
.form-message.success div { display: grid; flex: 1; gap: 2px; }
.form-message.success button { min-height: 34px; padding: 0 11px; color: #235d37; background: transparent; border: 1px solid #77a888; border-radius: 3px; cursor: pointer; }
.spinner { animation: spin .85s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 700px) {
  .showtime-layout { width: calc(100% - 28px); padding-top: 30px; }
  .form-section { padding: 22px 18px; }
  .two-columns, .four-columns { grid-template-columns: 1fr; }
  .wide { grid-column: auto; }
  .form-actions { padding: 18px; }
  .form-message.success { align-items: start; flex-wrap: wrap; }
}
@media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
</style>

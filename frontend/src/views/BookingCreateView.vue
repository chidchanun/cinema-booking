<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Armchair, Check, Clock3, LoaderCircle, Ticket, XCircle } from '@lucide/vue'
import { useRouter } from 'vue-router'

import SiteHeader from '@/components/SiteHeader.vue'
import { ApiError } from '@/services/api'
import {
  confirmManyBookings,
  getSeatMap,
  getShowtime,
  lockSeat,
  releaseSeat,
  seatRealtimeURL,
  sendPaymentReminder,
  type SeatLock,
  type SeatMap,
  type SeatRealtimeEvent,
} from '@/services/booking'
import { getMovie } from '@/services/movies'
import {
  getPendingReservation,
  removePendingReservation,
  savePendingReservation,
} from '@/services/pendingReservations'
import type { Movie, Showtime } from '@/types/movie'

const props = defineProps<{ showtimeId: string }>()
const router = useRouter()
const showtime = ref<Showtime | null>(null)
const movie = ref<Movie | null>(null)
const seatMap = ref<SeatMap | null>(null)
const selectedLocks = ref<SeatLock[]>([])
const loading = ref(true)
const selecting = ref(false)
const confirming = ref(false)
const errorMessage = ref('')
const paymentStarted = ref(false)
const sendingReminder = ref(false)
const cancellingReservation = ref(false)
const emailWarning = ref('')
const now = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | undefined
let socket: WebSocket | undefined
let reconnectTimer: ReturnType<typeof setTimeout> | undefined
let allowReconnect = true

const lockStorageKey = computed(() => `cinema.seat_lock.${props.showtimeId}`)

const seatRows = computed(() => {
  const rows = new Map<string, SeatMap['seats']>()
  for (const seat of seatMap.value?.seats ?? []) {
    rows.set(seat.row, [...(rows.get(seat.row) ?? []), seat])
  }
  return [...rows.entries()]
})

const lockSeconds = computed(() => {
  if (selectedLocks.value.length === 0) return 0
  const earliestExpiry = Math.min(
    ...selectedLocks.value.map((lock) => new Date(lock.expires_at).getTime()),
  )
  return Math.max(0, Math.ceil((earliestExpiry - now.value) / 1000))
})

const selectedSeatCodes = computed(() => selectedLocks.value.map((lock) => lock.seat_code))

const formattedCountdown = computed(() => {
  const minutes = Math.floor(lockSeconds.value / 60)
  const seconds = lockSeconds.value % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
})

const seatGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${showtime.value?.seats_per_row ?? 1}, 38px)`,
}))

function formatShowtime(value: string): string {
  const result = new Intl.DateTimeFormat('th-TH', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
  return `${result} น.`
}

function formatPrice(value: number, currency: string): string {
  return new Intl.NumberFormat('th-TH', {
    style: 'currency',
    currency,
    maximumFractionDigits: 0,
  }).format(value)
}

function saveLocks(): void {
  if (selectedLocks.value.length === 0) {
    clearSavedLock()
    return
  }
  sessionStorage.setItem(lockStorageKey.value, JSON.stringify(selectedLocks.value))
}

function clearSavedLock(): void {
  sessionStorage.removeItem(lockStorageKey.value)
}

function readSavedLocks(): SeatLock[] {
  try {
    const raw = sessionStorage.getItem(lockStorageKey.value)
    if (!raw) return []
    const parsed = JSON.parse(raw) as SeatLock | SeatLock[]
    return Array.isArray(parsed) ? parsed : [parsed]
  } catch {
    clearSavedLock()
    return []
  }
}

function restoreSavedLocks(): void {
  const locks = readSavedLocks()
  selectedLocks.value = locks.filter((lock) => new Date(lock.expires_at).getTime() > Date.now())
  if (selectedLocks.value.length === 0) {
    clearSavedLock()
    removePendingReservation(props.showtimeId)
  }
  paymentStarted.value = getPendingReservation(props.showtimeId) !== null
}

function applySeatEvent(event: SeatRealtimeEvent): void {
  if (!seatMap.value || event.showtime_id !== props.showtimeId) return
  const seats = seatMap.value.seats.map((seat) =>
    seat.code === event.seat_code ? { ...seat, status: event.status } : seat,
  )
  seatMap.value = {
    ...seatMap.value,
    seats,
    summary: {
      total: seats.length,
      available: seats.filter((seat) => seat.status === 'AVAILABLE').length,
      locked: seats.filter((seat) => seat.status === 'LOCKED').length,
      booked: seats.filter((seat) => seat.status === 'BOOKED').length,
    },
  }
}

function connectRealtime(): void {
  socket?.close()
  socket = new WebSocket(seatRealtimeURL(props.showtimeId))
  socket.onmessage = (message) => {
    try {
      applySeatEvent(JSON.parse(message.data as string) as SeatRealtimeEvent)
    } catch {
      // Ignore an invalid event without dropping the live connection.
    }
  }
  socket.onclose = () => {
    if (allowReconnect) reconnectTimer = setTimeout(connectRealtime, 2000)
  }
}

async function loadBooking(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    restoreSavedLocks()
    const [showtimeResult, seatsResult] = await Promise.all([
      getShowtime(props.showtimeId),
      getSeatMap(props.showtimeId),
    ])
    showtime.value = showtimeResult
    seatMap.value = seatsResult
    movie.value = await getMovie(showtimeResult.movie_id)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'ไม่สามารถโหลดข้อมูลรอบฉายได้'
  } finally {
    loading.value = false
  }
}

async function chooseSeat(seatCode: string): Promise<void> {
  if (selecting.value || confirming.value) return
  selecting.value = true
  errorMessage.value = ''
  paymentStarted.value = false
  emailWarning.value = ''

  try {
    const existingLock = selectedLocks.value.find((lock) => lock.seat_code === seatCode)
    if (existingLock) {
      await releaseSeat(props.showtimeId, existingLock.seat_code, existingLock.lock_id)
      selectedLocks.value = selectedLocks.value.filter((lock) => lock.seat_code !== seatCode)
      saveLocks()
    } else {
      if (selectedLocks.value.length >= 10) {
        errorMessage.value = 'เลือกได้สูงสุด 10 ที่นั่งต่อการจอง'
        return
      }
      const acquiredLock = await lockSeat(props.showtimeId, seatCode)
      selectedLocks.value = [...selectedLocks.value, acquiredLock]
      saveLocks()
    }
    now.value = Date.now()
    seatMap.value = await getSeatMap(props.showtimeId)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'ไม่สามารถเลือกที่นั่งได้'
    seatMap.value = await getSeatMap(props.showtimeId).catch(() => seatMap.value)
  } finally {
    selecting.value = false
  }
}

async function beginPayment(): Promise<void> {
  if (selectedLocks.value.length === 0 || sendingReminder.value) return
  sendingReminder.value = true
  emailWarning.value = ''
  const expiresAt = selectedLocks.value.reduce((earliest, lock) =>
      new Date(lock.expires_at).getTime() < new Date(earliest).getTime()
        ? lock.expires_at
        : earliest,
    selectedLocks.value[0]!.expires_at)
  if (showtime.value) {
    savePendingReservation({
      showtime_id: props.showtimeId,
      movie_id: showtime.value.movie_id,
      movie_title: movie.value?.title ?? 'Movie',
      hall_name: showtime.value.hall_name,
      showtime_start: showtime.value.start_time,
      price: showtime.value.price,
      currency: showtime.value.currency,
      locks: [...selectedLocks.value],
      expires_at: expiresAt,
    })
  }
  try {
    await sendPaymentReminder({
      showtime_id: props.showtimeId,
      seat_codes: selectedSeatCodes.value,
      expires_at: expiresAt,
      status: 'PENDING',
    })
  } catch (error) {
    emailWarning.value =
      error instanceof ApiError
        ? `${error.message} แต่ยังสามารถทดลองชำระเงินได้`
        : 'ส่งอีเมลแจ้งเตือนไม่สำเร็จ แต่ยังสามารถทดลองชำระเงินได้'
  } finally {
    paymentStarted.value = true
    sendingReminder.value = false
  }
}

async function cancelReservation(): Promise<void> {
  if (selectedLocks.value.length === 0 || cancellingReservation.value || confirming.value) return
  cancellingReservation.value = true
  errorMessage.value = ''
  const locks = [...selectedLocks.value]
  try {
    await Promise.all(
      locks.map((lock) => releaseSeat(props.showtimeId, lock.seat_code, lock.lock_id)),
    )
    selectedLocks.value = []
    clearSavedLock()
    removePendingReservation(props.showtimeId)
    paymentStarted.value = false
    emailWarning.value = ''
    seatMap.value = await getSeatMap(props.showtimeId)
  } catch (error) {
    errorMessage.value =
      error instanceof ApiError ? error.message : 'ไม่สามารถยกเลิกการจองที่นั่งได้'
  } finally {
    cancellingReservation.value = false
  }
}

async function submitBooking(): Promise<void> {
  if (selectedLocks.value.length === 0 || confirming.value || lockSeconds.value === 0) return
  confirming.value = true
  errorMessage.value = ''
  try {
    const paidSeatCodes = [...selectedSeatCodes.value]
    const expiresAt = selectedLocks.value[0]!.expires_at
    await confirmManyBookings({
      showtime_id: props.showtimeId,
      seats: selectedLocks.value.map((lock) => ({
        seat_code: lock.seat_code,
        lock_id: lock.lock_id,
      })),
    })
    clearSavedLock()
    removePendingReservation(props.showtimeId)
    selectedLocks.value = []
    await sendPaymentReminder({
      showtime_id: props.showtimeId,
      seat_codes: paidSeatCodes,
      expires_at: expiresAt,
      status: 'PAID',
    }).catch(() => undefined)
    await router.replace('/bookings')
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'ไม่สามารถยืนยันการจองได้'
  } finally {
    confirming.value = false
  }
}

onMounted(() => {
  void loadBooking()
  connectRealtime()
  clockTimer = setInterval(() => {
    now.value = Date.now()
    if (selectedLocks.value.length > 0 && lockSeconds.value === 0) {
      selectedLocks.value = selectedLocks.value.filter(
        (lock) => new Date(lock.expires_at).getTime() > now.value,
      )
      saveLocks()
      errorMessage.value = 'หมดเวลาเลือกที่นั่ง กรุณาเลือกใหม่อีกครั้ง'
      void getSeatMap(props.showtimeId).then((result) => (seatMap.value = result))
    }
  }, 1000)
})

onBeforeUnmount(() => {
  allowReconnect = false
  if (reconnectTimer) clearTimeout(reconnectTimer)
  socket?.close()
  if (clockTimer) clearInterval(clockTimer)
  // Keep an unpaid reservation alive when navigating away. Redis TTL remains
  // authoritative and releases the seats automatically when payment expires.
})
</script>

<template>
  <main class="booking-page">
    <SiteHeader :back-to="showtime ? `/movies/${showtime.movie_id}` : '/'" back-label="กลับไปเลือกรอบฉาย" />

    <div v-if="loading" class="booking-status"><LoaderCircle class="spinner" :size="30" />กำลังโหลดผังที่นั่ง</div>
    <div v-else-if="!showtime || !seatMap" class="booking-status error" role="alert"><p>{{ errorMessage || 'ไม่พบรอบฉาย' }}</p></div>

    <div v-else class="booking-layout">
      <header class="booking-heading">
        <div><p>SELECT YOUR SEAT</p><h1>{{ movie?.title || 'เลือกที่นั่ง' }}</h1><span>{{ formatShowtime(showtime.start_time) }} · {{ showtime.hall_name }}</span></div>
        <div v-if="selectedLocks.length" class="lock-timer"><Clock3 :size="19" /><span>เวลาที่เหลือ · {{ selectedLocks.length }} ที่นั่ง</span><strong>{{ formattedCountdown }}</strong></div>
      </header>

      <p v-if="errorMessage" class="booking-error" role="alert">{{ errorMessage }}</p>

      <div class="booking-workspace">
        <section class="seat-area" aria-label="ผังที่นั่ง">
          <div class="screen"><span>จอภาพยนตร์</span></div>
          <div class="seat-map-scroll">
            <div class="seat-map">
              <div v-for="[row, seats] in seatRows" :key="row" class="seat-row">
                <span class="row-label">{{ row }}</span>
                <div class="seat-grid" :style="seatGridStyle">
                  <button
                    v-for="seat in seats"
                    :key="seat.code"
                    class="seat"
                    :class="[seat.status.toLowerCase(), { selected: selectedSeatCodes.includes(seat.code) }]"
                    type="button"
                    :disabled="(seat.status !== 'AVAILABLE' && !selectedSeatCodes.includes(seat.code)) || selecting"
                    :aria-label="`ที่นั่ง ${seat.code} ${seat.status}`"
                    :title="`ที่นั่ง ${seat.code}`"
                    @click="chooseSeat(seat.code)"
                  >
                    <Armchair :size="20" />
                    <span>{{ seat.number }}</span>
                  </button>
                </div>
                <span class="row-label">{{ row }}</span>
              </div>
            </div>
          </div>
          <div class="seat-legend"><span><i class="available-dot"></i>ว่าง</span><span><i class="selected-dot"></i>เลือกแล้ว</span><span><i class="locked-dot"></i>กำลังถูกเลือก</span><span><i class="booked-dot"></i>จองแล้ว</span></div>
        </section>

        <aside class="booking-summary">
          <div class="summary-heading"><Ticket :size="20" /><div><h2>สรุปการจอง</h2><span class="payment-status">{{ paymentStarted ? 'รอชำระเงิน' : 'เลือกที่นั่งแล้ว' }}</span></div></div>
          <dl><div><dt>ภาพยนตร์</dt><dd>{{ movie?.title }}</dd></div><div><dt>รอบฉาย</dt><dd>{{ formatShowtime(showtime.start_time) }}</dd></div><div><dt>Hall</dt><dd>{{ showtime.hall_name }}</dd></div><div><dt>ที่นั่ง</dt><dd class="selected-seat">{{ selectedSeatCodes.length ? selectedSeatCodes.join(', ') : 'ยังไม่ได้เลือก' }}</dd></div></dl>
          <div class="total"><span>ยอดชำระ · {{ selectedLocks.length }} ที่นั่ง</span><strong>{{ formatPrice(showtime.price * selectedLocks.length, showtime.currency) }}</strong></div>
          <button v-if="!paymentStarted" class="confirm-button" type="button" :disabled="selectedLocks.length === 0 || sendingReminder || lockSeconds === 0" @click="beginPayment"><LoaderCircle v-if="sendingReminder" class="spinner" :size="19" /><Ticket v-else :size="19" />{{ sendingReminder ? 'กำลังส่งอีเมล' : 'ดำเนินการชำระเงิน' }}</button>
          <button v-else class="confirm-button payment-button" type="button" :disabled="confirming || lockSeconds === 0" @click="submitBooking"><LoaderCircle v-if="confirming" class="spinner" :size="19" /><Check v-else :size="19" />{{ confirming ? 'กำลังชำระเงิน' : `จำลองชำระ ${formatPrice(showtime.price * selectedLocks.length, showtime.currency)}` }}</button>
          <button v-if="selectedLocks.length" class="cancel-reservation" type="button" :disabled="cancellingReservation || confirming" @click="cancelReservation"><LoaderCircle v-if="cancellingReservation" class="spinner" :size="18" /><XCircle v-else :size="18" />{{ cancellingReservation ? 'กำลังคืนที่นั่ง' : 'ยกเลิกการจอง' }}</button>
          <p v-if="emailWarning" class="email-warning" role="alert">{{ emailWarning }}</p>
          <p>หากไม่ชำระภายในเวลาที่กำหนด ระบบจะปล่อยที่นั่งทั้งหมดอัตโนมัติ</p>
        </aside>
      </div>
    </div>
  </main>
</template>

<style scoped>
.booking-page { min-height: 100vh; color: #27231f; background: #efede9; }
.booking-layout { width: min(1180px, calc(100% - 48px)); margin: 0 auto; padding: 42px 0 70px; }
.booking-heading { display: flex; align-items: end; justify-content: space-between; gap: 24px; padding-bottom: 24px; border-bottom: 1px solid #cec9c2; }
.booking-heading p { margin: 0 0 8px; color: #926e22; font-size: 13px; font-weight: 800; }
.booking-heading h1 { margin: 0; font-family: Georgia, "Times New Roman", serif; font-size: 42px; font-weight: 500; letter-spacing: 0; }
.booking-heading span { display: block; margin-top: 8px; color: #716a63; font-size: 15px; }
.lock-timer { display: grid; grid-template-columns: auto auto; gap: 2px 9px; min-width: 150px; padding: 11px 14px; color: #704f0d; background: #fff5dc; border: 1px solid #ddc27f; }
.lock-timer svg { grid-row: 1 / 3; align-self: center; }
.lock-timer span { margin: 0; font-size: 12px; }
.lock-timer strong { font-size: 20px; }
.booking-workspace { display: grid; grid-template-columns: minmax(0, 1fr) 310px; gap: 24px; margin-top: 24px; align-items: start; }
.seat-area { min-width: 0; padding: 28px; background: #fff; border: 1px solid #d5d0c9; }
.screen { width: min(560px, 80%); height: 34px; margin: 0 auto 44px; text-align: center; border-top: 5px solid #b99850; border-radius: 50%; box-shadow: 0 -8px 18px rgba(185, 152, 80, .18); }
.screen span { position: relative; top: 10px; color: #8a8279; font-size: 11px; }
.seat-map-scroll { overflow-x: auto; padding-bottom: 8px; }
.seat-map { width: max-content; min-width: 100%; }
.seat-row { display: grid; grid-template-columns: 22px auto 22px; justify-content: center; align-items: center; gap: 10px; margin-bottom: 8px; }
.seat-grid { display: grid; gap: 7px; }
.row-label { color: #857e76; font-size: 12px; font-weight: 750; text-align: center; }
.seat { position: relative; display: grid; width: 38px; height: 38px; place-items: center; padding: 0; color: #67615a; background: #f5f3ef; border: 1px solid #c9c3bb; border-radius: 3px; cursor: pointer; }
.seat > span { position: absolute; bottom: 1px; font-size: 9px; }
.seat:hover:not(:disabled), .seat.selected { color: #fff; background: #9b752a; border-color: #795817; }
.seat.locked { color: #9a7021; background: #f9edce; border-color: #d8bd78; }
.seat.booked { color: #aaa39b; background: #dedbd6; border-color: #d0cbc5; }
.seat.locked.selected { color: #fff; background: #9b752a; border-color: #795817; }
.seat:disabled { cursor: not-allowed; }
.seat.selected { cursor: default; }
.seat-legend { display: flex; justify-content: center; gap: 20px; margin-top: 24px; color: #6f6962; font-size: 12px; }
.seat-legend span { display: flex; align-items: center; gap: 6px; }
.seat-legend i { width: 12px; height: 12px; border: 1px solid; border-radius: 2px; }
.available-dot { background: #f5f3ef; border-color: #c9c3bb !important; }
.selected-dot { background: #9b752a; border-color: #795817 !important; }
.locked-dot { background: #f9edce; border-color: #d8bd78 !important; }
.booked-dot { background: #dedbd6; border-color: #d0cbc5 !important; }
.booking-summary { padding: 22px; background: #fff; border: 1px solid #d5d0c9; }
.summary-heading { display: flex; align-items: center; gap: 9px; padding-bottom: 16px; border-bottom: 1px solid #e0dcd6; }
.summary-heading svg { color: #9b752a; }
.summary-heading h2 { margin: 0; font-size: 18px; }
.summary-heading > div { display: grid; gap: 3px; }
.payment-status { color: #9a6e17; font-size: 11px; font-weight: 700; }
.booking-summary dl { margin: 8px 0 0; }
.booking-summary dl > div { display: grid; grid-template-columns: 90px 1fr; gap: 8px; padding: 10px 0; border-bottom: 1px solid #eeeae5; }
.booking-summary dt { color: #7c756e; font-size: 12px; }
.booking-summary dd { margin: 0; font-size: 13px; font-weight: 650; text-align: right; overflow-wrap: anywhere; }
.selected-seat { color: #8d681e; font-size: 18px !important; }
.total { display: flex; align-items: end; justify-content: space-between; padding: 18px 0; }
.total span { color: #655f58; font-size: 13px; }
.total strong { font-size: 23px; }
.confirm-button { display: flex; width: 100%; min-height: 45px; align-items: center; justify-content: center; gap: 8px; color: #fff; font-weight: 750; background: #292622; border: 0; border-radius: 4px; cursor: pointer; }
.confirm-button:disabled { cursor: not-allowed; opacity: .5; }
.payment-button { background: #8b681f; }
.cancel-reservation { display: flex; width: 100%; min-height: 41px; align-items: center; justify-content: center; gap: 8px; margin-top: 8px; color: #92332c; font-weight: 700; background: #fff; border: 1px solid #d2a5a1; border-radius: 4px; cursor: pointer; }
.cancel-reservation:hover:not(:disabled) { background: #fdf0ef; }
.cancel-reservation:disabled { cursor: wait; opacity: .6; }
.booking-summary .email-warning { color: #982f28; text-align: left; }
.booking-summary > p { margin: 12px 0 0; color: #817a72; font-size: 11px; line-height: 1.5; text-align: center; }
.booking-error { margin: 16px 0 0; padding: 11px 13px; color: #942e27; background: #fbe9e7; border-left: 3px solid #b43a32; }
.booking-status { display: flex; min-height: calc(100vh - 72px); align-items: center; justify-content: center; gap: 10px; color: #716a63; }
.booking-status.error { color: #942e27; }
.spinner { animation: spin .85s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 850px) { .booking-workspace { grid-template-columns: 1fr; } .booking-summary { position: static; } }
@media (max-width: 600px) { .booking-layout { width: calc(100% - 28px); padding-top: 28px; } .booking-heading { align-items: start; } .booking-heading h1 { font-size: 34px; } .seat-area { padding: 20px 12px; } .seat-legend { flex-wrap: wrap; gap: 10px 16px; } .lock-timer { min-width: 125px; } }
@media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
</style>

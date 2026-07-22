import { apiRequest } from '@/services/api'
import type { Showtime } from '@/types/movie'
import type { Booking } from '@/services/bookings'

export interface Seat {
  code: string
  row: string
  number: number
  status: 'AVAILABLE' | 'LOCKED' | 'BOOKED'
}

export interface SeatMap {
  showtime_id: string
  movie_id: string
  hall_name: string
  start_time: string
  price: number
  currency: string
  status: 'ACTIVE' | 'CANCELLED'
  summary: { total: number; available: number; locked: number; booked: number }
  seats: Seat[]
}

export interface SeatLock {
  showtime_id: string
  seat_code: string
  lock_id: string
  expires_at: string
  expires_in: number
  already_owned: boolean
}

export interface SeatRealtimeEvent {
  type: 'seat.status_changed'
  showtime_id: string
  seat_code: string
  status: Seat['status']
  expires_at?: string
  booking_id?: string
  occurred_at: string
}

export function getShowtime(showtimeID: string): Promise<Showtime> {
  return apiRequest(`/showtimes/${encodeURIComponent(showtimeID)}`)
}

export function getSeatMap(showtimeID: string): Promise<SeatMap> {
  return apiRequest(`/showtimes/${encodeURIComponent(showtimeID)}/seats`)
}

export function lockSeat(showtimeID: string, seatCode: string): Promise<SeatLock> {
  return apiRequest(
    `/showtimes/${encodeURIComponent(showtimeID)}/seats/${encodeURIComponent(seatCode)}/lock`,
    { method: 'POST' },
  )
}

export function releaseSeat(
  showtimeID: string,
  seatCode: string,
  lockID: string,
): Promise<{ message: string }> {
  return apiRequest(
    `/showtimes/${encodeURIComponent(showtimeID)}/seats/${encodeURIComponent(seatCode)}/lock`,
    { method: 'DELETE', headers: { 'X-Seat-Lock-Token': lockID } },
  )
}

export function confirmBooking(input: {
  showtime_id: string
  seat_code: string
  lock_id: string
}): Promise<Booking> {
  return apiRequest('/bookings/confirm', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function confirmManyBookings(input: {
  showtime_id: string
  seats: Array<{ seat_code: string; lock_id: string }>
}): Promise<{ data: Booking[] }> {
  return apiRequest('/bookings/confirm-many', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function sendPaymentReminder(input: {
  showtime_id: string
  seat_codes: string[]
  expires_at: string
  status: 'PENDING' | 'PAID'
}): Promise<{ message: string }> {
  return apiRequest('/bookings/payment-reminder', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function seatRealtimeURL(showtimeID: string): string {
  const apiBase = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  const url = new URL(apiBase, window.location.origin)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.pathname = `${url.pathname.replace(/\/$/, '')}/ws/showtimes/${encodeURIComponent(showtimeID)}/seats`
  url.search = ''
  return url.toString()
}

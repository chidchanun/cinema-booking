import { apiRequest } from '@/services/api'

export interface Booking {
  id: string
  booking_code: string
  movie_id: string
  showtime_id: string
  seat_code: string
  hall_name: string
  showtime_start: string
  price: number
  currency: string
  status: 'BOOKED' | 'CANCELLED'
  confirmed_at: string
  created_at: string
}

export interface BookingListResponse {
  data: Booking[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export function listMyBookings(): Promise<BookingListResponse> {
  return apiRequest<BookingListResponse>('/bookings?page=1&limit=100')
}

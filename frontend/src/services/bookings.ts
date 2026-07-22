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

export interface MyBookingFilters {
  page?: number
  limit?: number
  movieID?: string
  from?: string
  to?: string
}

export function listMyBookings(filters: MyBookingFilters = {}): Promise<BookingListResponse> {
  const query = new URLSearchParams({
    page: String(filters.page ?? 1),
    limit: String(filters.limit ?? 10),
  })
  if (filters.movieID) query.set('movie_id', filters.movieID)
  if (filters.from) query.set('from', filters.from)
  if (filters.to) query.set('to', filters.to)
  return apiRequest<BookingListResponse>(`/bookings?${query.toString()}`)
}

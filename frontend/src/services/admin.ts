import { apiRequest } from '@/services/api'
import type { Movie, MovieListResponse, Showtime } from '@/types/movie'

export interface MovieInput {
  title: string
  description: string
  duration_minutes: number
  poster_url: string
  is_active: boolean
}

export interface ShowtimeInput {
  movie_id: string
  hall_name: string
  start_time: string
  price: number
  currency: string
  seat_rows: number
  seats_per_row: number
}

export interface HallSummary {
  name: string
  seat_rows: number
  seats_per_row: number
  total_seats: number
}

export interface AdminBooking {
  id: string
  booking_code: string
  user: { id: string; name: string; email: string }
  movie: { id: string; title: string }
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

export interface AuditLog {
  id: string
  event_id: string
  event_type: string
  actor_type: 'USER' | 'SYSTEM'
  actor_user_id?: string
  entity_type: string
  entity_id?: string
  action: string
  severity: 'INFO' | 'WARNING' | 'ERROR'
  occurred_at: string
}

export interface TMDBMovie {
  id: number
  title: string
  overview: string
  poster_url: string
  release_date: string
  vote_average: number
  runtime?: number
}

export function getTMDBMovie(movieID: number): Promise<TMDBMovie> {
  return apiRequest(`/admin/tmdb/movies/${movieID}`)
}

export interface Paginated<T> {
  data: T[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export function listAdminMovies(search = '', page = 1, limit = 100): Promise<MovieListResponse> {
  const query = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (search.trim()) query.set('search', search.trim())
  return apiRequest<MovieListResponse>(`/admin/movies?${query.toString()}`)
}

export function createMovie(input: MovieInput): Promise<Movie> {
  return apiRequest<Movie>('/admin/movies', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateMovie(movieID: string, input: Partial<MovieInput>): Promise<Movie> {
  return apiRequest<Movie>(`/admin/movies/${encodeURIComponent(movieID)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteMovie(movieID: string): Promise<{ message: string }> {
  return apiRequest(`/admin/movies/${encodeURIComponent(movieID)}`, { method: 'DELETE' })
}

export function createShowtime(input: ShowtimeInput): Promise<Showtime> {
  return apiRequest('/admin/showtimes', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function listHalls(): Promise<{ data: HallSummary[] }> {
  return apiRequest('/admin/halls')
}

export function checkHallAvailability(input: {
  movieID: string
  hallName: string
  startTime: string
}): Promise<{ available: boolean; end_time: string }> {
  const query = new URLSearchParams({
    movie_id: input.movieID,
    hall_name: input.hallName.trim(),
    start_time: input.startTime,
  })
  return apiRequest(`/admin/halls/availability?${query.toString()}`)
}

export function cancelShowtime(showtimeID: string): Promise<{ message: string }> {
  return apiRequest(`/admin/showtimes/${encodeURIComponent(showtimeID)}`, {
    method: 'DELETE',
  })
}

export interface AdminBookingFilters {
  status?: string
  movieID?: string
  userID?: string
  from?: string
  to?: string
  page?: number
  limit?: number
}

export function listAdminBookings(filters: AdminBookingFilters = {}): Promise<Paginated<AdminBooking>> {
  const query = new URLSearchParams({
    page: String(filters.page ?? 1),
    limit: String(filters.limit ?? 10),
  })
  if (filters.status) query.set('status', filters.status)
  if (filters.movieID) query.set('movie_id', filters.movieID)
  if (filters.userID) query.set('user_id', filters.userID)
  if (filters.from) query.set('from', filters.from)
  if (filters.to) query.set('to', filters.to)
  return apiRequest(`/admin/bookings?${query.toString()}`)
}

export function listAuditLogs(action = '', page = 1, limit = 10): Promise<Paginated<AuditLog>> {
  const query = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (action) query.set('action', action)
  return apiRequest(`/admin/audit-logs?${query.toString()}`)
}

export function searchTMDBMovies(
  query: string,
  signal?: AbortSignal,
): Promise<{ data: TMDBMovie[] }> {
  return apiRequest(`/admin/tmdb/movies?query=${encodeURIComponent(query.trim())}`, { signal })
}

export interface Movie {
  id: string
  title: string
  description: string
  duration_minutes: number
  poster_url?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface MovieListResponse {
  data: Movie[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface Showtime {
  id: string
  movie_id: string
  hall_name: string
  start_time: string
  end_time: string
  price: number
  currency: string
  seat_rows: number
  seats_per_row: number
  total_seats: number
  status: 'ACTIVE' | 'CANCELLED'
}

export interface ShowtimeListResponse {
  data: Showtime[]
}

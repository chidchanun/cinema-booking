import { apiRequest } from './api'
import type { Movie, MovieListResponse, ShowtimeListResponse } from '@/types/movie'

export function listMovies(search = '', signal?: AbortSignal, limit = 24): Promise<MovieListResponse> {
  const query = new URLSearchParams({ page: '1', limit: String(limit) })
  if (search.trim()) query.set('search', search.trim())
  return apiRequest<MovieListResponse>(`/movies?${query.toString()}`, { signal })
}

export function getMovie(movieID: string): Promise<Movie> {
  return apiRequest<Movie>(`/movies/${encodeURIComponent(movieID)}`)
}

export function listMovieShowtimes(movieID: string): Promise<ShowtimeListResponse> {
  return apiRequest<ShowtimeListResponse>(
    `/movies/${encodeURIComponent(movieID)}/showtimes`,
  )
}

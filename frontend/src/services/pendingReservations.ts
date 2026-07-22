import type { SeatLock } from '@/services/booking'

const PREFIX = 'cinema.pending_reservation.'

export interface PendingReservation {
  showtime_id: string
  movie_title: string
  hall_name: string
  showtime_start: string
  price: number
  currency: string
  locks: SeatLock[]
  expires_at: string
}

export function savePendingReservation(reservation: PendingReservation): void {
  sessionStorage.setItem(`${PREFIX}${reservation.showtime_id}`, JSON.stringify(reservation))
}

export function removePendingReservation(showtimeID: string): void {
  sessionStorage.removeItem(`${PREFIX}${showtimeID}`)
}

export function getPendingReservation(showtimeID: string): PendingReservation | null {
  const raw = sessionStorage.getItem(`${PREFIX}${showtimeID}`)
  if (!raw) return null
  try {
    const reservation = JSON.parse(raw) as PendingReservation
    if (new Date(reservation.expires_at).getTime() <= Date.now()) {
      removePendingReservation(showtimeID)
      return null
    }
    return reservation
  } catch {
    removePendingReservation(showtimeID)
    return null
  }
}

export function listPendingReservations(): PendingReservation[] {
  const reservations: PendingReservation[] = []
  for (let index = 0; index < sessionStorage.length; index += 1) {
    const key = sessionStorage.key(index)
    if (!key?.startsWith(PREFIX)) continue
    const reservation = getPendingReservation(key.slice(PREFIX.length))
    if (reservation) reservations.push(reservation)
  }
  return reservations.sort(
    (left, right) => new Date(left.expires_at).getTime() - new Date(right.expires_at).getTime(),
  )
}

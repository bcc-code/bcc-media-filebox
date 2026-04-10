import { ulid } from 'ulid'

const STORAGE_KEY = 'file-pusher-user-id'

let cached: string | null = null

export function getUserId(): string {
  if (cached) return cached
  let id = localStorage.getItem(STORAGE_KEY)
  if (!id) {
    id = ulid()
    localStorage.setItem(STORAGE_KEY, id)
  }
  cached = id
  return id
}

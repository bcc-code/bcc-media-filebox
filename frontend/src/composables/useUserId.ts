import { ulid } from 'ulid'
import { useAuth } from './useAuth'

const STORAGE_KEY = 'filebox-user-id'

let cachedGuestId: string | null = null

function getGuestId(): string {
  if (cachedGuestId) return cachedGuestId
  let id = localStorage.getItem(STORAGE_KEY)
  if (!id) {
    id = ulid()
    localStorage.setItem(STORAGE_KEY, id)
  }
  cachedGuestId = id
  return id
}

// getUserId returns the canonical user_id for this caller:
//   guest:<ulid>            when not signed in
//   <provider>:<subject>    when signed in (mirrors the value the server enforces)
export function getUserId(): string {
  const { state } = useAuth()
  if (state.authenticated && state.userId) return state.userId
  return `guest:${getGuestId()}`
}

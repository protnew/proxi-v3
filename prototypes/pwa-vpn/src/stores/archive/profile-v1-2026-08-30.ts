/**
 * PWA-004: Nano Store — profile (identity)
 */
import { atom } from 'nanostores'
import { persistentAtom } from '@nanostores/persistent'

export interface Profile {
  pubkey: string
  npub: string
  name: string
  avatar?: string
}

export const profileStore = persistentAtom<Profile>('proxi-profile', {
  pubkey: '',
  npub: '',
  name: 'User',
}, {
  encode: JSON.stringify,
  decode: JSON.parse,
})

export function setProfile(p: Partial<Profile>) {
  profileStore.set({ ...profileStore.get(), ...p })
}

export function clearProfile() {
  profileStore.set({ pubkey: '', npub: '', name: 'User' })
}

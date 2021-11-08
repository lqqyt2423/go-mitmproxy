import { IRequest, IResponse } from './message'

export const isTextBody = (payload: IRequest | IResponse) => {
  if (!payload) return false
  if (!payload.header) return false
  if (!payload.header['Content-Type']) return false

  return /text|javascript|json|x-www-form-urlencoded|xml|form-data/.test(payload.header['Content-Type'].join(''))
}

export const getSize = (len: number) => {
  if (!len) return '0'
  if (isNaN(len)) return '0'
  if (len <= 0) return '0'

  if (len < 1024) return `${len} B`
  if (len < 1024 * 1024) return `${(len / 1024).toFixed(2)} KB`
  return `${(len / (1024 * 1024)).toFixed(2)} MB`
}

export const shallowEqual = (objA: any, objB: any) => {
  if (objA === objB) return true

  const keysA = Object.keys(objA)
  const keysB = Object.keys(objB)
  if (keysA.length !== keysB.length) return false

  for (let i = 0; i < keysA.length; i++) {
    const key = keysA[i]
    if (objB[key] === undefined || objA[key] !== objB[key]) return false
  }
  return true
}

export const arrayBufferToBase64 = (buf: ArrayBuffer) => {
  let binary = ''
  const bytes = new Uint8Array(buf)
  const len = bytes.byteLength
  for (let i = 0; i < len; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

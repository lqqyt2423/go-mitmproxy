import type { IRequest, IResponse, Header } from './flow'

export const getHeader = (header: Header, key: string) => {
  return header[key] || header[key.toLowerCase()] || null
}

export const setHeader = (header: Header, key: string, value: string[]) => {
  if (header[key]) {
    header[key] = value
    return
  }

  if (header[key.toLowerCase()]) {
    header[key.toLowerCase()] = value
    return
  }

  header[key] = value
}

export const hasHeader = (header: Header, key: string) => {
  return getHeader(header, key) != null
}

export const delHeader = (header: Header, key: string) => {
  if (header[key]) {
    delete header[key]
  }

  if (header[key.toLowerCase()]) {
    delete header[key.toLowerCase()]
  }
}

export const flattenHeader = (header: Header) => {
  return Object.keys(header).reduce((res, key) => {
    return res.concat(header[key].map(value => ({ key, value })))
  }, [] as Array<{ key: string; value: string }>)
}

export const isTextBody = (payload: IRequest | IResponse) => {
  if (!payload) return false
  if (!payload.header) return false
  if (!hasHeader(payload.header, 'Content-Type')) return false

  return /text|javascript|json|x-www-form-urlencoded|xml|form-data/.test(getHeader(payload.header, 'Content-Type').join(''))
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

export const bufHexView = (buf: ArrayBuffer) => {
  let str = ''
  const bytes = new Uint8Array(buf)
  const len = bytes.byteLength

  let viewStr = ''

  str += '00000000:  '
  for (let i = 0; i < len; i++) {
    str += bytes[i].toString(16).padStart(2, '0') + ' '

    if (bytes[i] >= 32 && bytes[i] <= 126) {
      viewStr += String.fromCharCode(bytes[i])
    } else {
      viewStr += '.'
    }

    if ((i + 1) % 16 === 0) {
      str += '   ' + viewStr
      viewStr = ''
      str += `\n${(i + 1).toString(16).padStart(8, '0')}:  `
    } else if ((i + 1) % 8 === 0) {
      str += '  '
    }
  }

  // 补充最后一行的空白
  if (viewStr.length > 0) {
    for (let i = viewStr.length; i < 16; i++) {
      str += '  ' + ' '
      if ((i + 1) % 8 === 0) str += '  '
    }
    str += ' ' + viewStr
  }

  return str
}

// https://github.com/febobo/web-interview/issues/84
export function isInViewPort(element: HTMLElement) {
  const viewWidth = window.innerWidth || document.documentElement.clientWidth
  const viewHeight = window.innerHeight || document.documentElement.clientHeight
  const {
    top,
    right,
    bottom,
    left,
  } = element.getBoundingClientRect()

  return (
    top >= 0 &&
    left >= 0 &&
    right <= viewWidth &&
    bottom <= viewHeight
  )
}

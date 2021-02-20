export const isTextResponse = response => {
  if (!response) return false
  if (!response.header) return false
  if (!response.header['Content-Type']) return false

  return /text|javascript|json/.test(response.header['Content-Type'].join(''))
}

export const getSize = response => {
  if (!response) return '0'
  if (!response.header) return '0'

  let len
  if (response.header['Content-Length']) {
    len = parseInt(response.header['Content-Length'][0])
  } else if (response && response.body) {
    len = response.body.byteLength
  }
  if (!len) return '0'
  if (isNaN(len)) return '0'
  if (len <= 0) return '0'
  
  if (len < 1024) return `${len} B`
  if (len < 1024*1024) return `${(len/1024).toFixed(2)} KB`
  return `${(len/(1024*1024)).toFixed(2)} MB`
}

export const parseMessage = data => {
  if (data.byteLength < 39) return null
  const meta = new Int8Array(data.slice(0, 3))
  const version = meta[0]
  if (version !== 1) return null
  const type = meta[1]
  if (![1, 2, 3].includes(type)) return null
  const id = new TextDecoder().decode(data.slice(3, 39))

  const resp = {
    type: ['request', 'response', 'responseBody'][type-1],
    id,
    waitIntercept: meta[2] === 1,
  }
  if (data.byteLength === 39) return resp
  if (type === 3) {
    resp.content = data.slice(39)
    return resp
  }

  let content = new TextDecoder().decode(data.slice(39))
  try {
    content = JSON.parse(content)
  } catch (err) {
    return null
  }

  resp.content = content
  return resp
}

/**
 * 
 * @param {number} messageType 
 * @param {string} id 
 * @param {string} content 
 */
export const buildMessage = (messageType, id, content) => {
  content = new TextEncoder().encode(content)
  const data = new Uint8Array(39 + content.byteLength)
  data[0] = 1
  data[1] = messageType
  data[2] = 0
  data.set(new TextEncoder().encode(id), 3)
  data.set(content, 39)
  return data
}

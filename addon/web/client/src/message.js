const messageEnum = {
  'request': 1,
  'requestBody': 2,
  'response': 3,
  'responseBody': 4,
}

const allMessageBytes = Object.keys(messageEnum).map(k => messageEnum[k])

const messageByteMap = Object.keys(messageEnum).reduce((m, k) => {
  m[messageEnum[k]] = k
  return m
}, {})


// type: 1/2/3/4
// messageFlow
// version 1 byte + type 1 byte + id 36 byte + waitIntercept 1 byte + content left bytes
export const parseMessage = data => {
  if (data.byteLength < 39) return null
  const meta = new Int8Array(data.slice(0, 39))
  const version = meta[0]
  if (version !== 1) return null
  const type = meta[1]
  if (!allMessageBytes.includes(type)) return null
  const id = new TextDecoder().decode(data.slice(2, 38))
  const waitIntercept = meta[38] === 1

  const resp = {
    type: messageByteMap[type],
    id,
    waitIntercept,
  }
  if (data.byteLength === 39) return resp
  if (type === messageEnum['requestBody'] || type === messageEnum['responseBody']) {
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


export const sendMessageEnum = {
  'changeRequest': 11,
  'changeResponse': 12,
  'changeBreakPointRules': 21,
}

// type: 11/12
// messageEdit
// version 1 byte + type 1 byte + id 36 byte + header len 4 byte + header content bytes + body len 4 byte + [body content bytes]
export const buildMessageEdit = (messageType, flow) => {
  let header, body
  
  if (messageType === sendMessageEnum.changeRequest) {
    ({ body, ...header } = flow.request)
  } else if (messageType === sendMessageEnum.changeResponse) {
    ({ body, ...header } = flow.response)
  } else {
    throw new Error('invalid message type')
  }
  
  const bodyLen = (body && body.byteLength) ? body.byteLength : 0
  const headerBytes = new TextEncoder().encode(JSON.stringify(header))
  const len = 2 + 36 + 4 + headerBytes.byteLength + 4 + bodyLen
  const data = new ArrayBuffer(len)
  const view = new Uint8Array(data)
  view[0] = 1
  view[1] = messageType
  view.set(new TextEncoder().encode(flow.id), 2)
  view.set(headerBytes, 2 + 36 + 4)
  if (bodyLen) view.set(body, 2 + 36 + 4 + headerBytes.byteLength + 4)

  const view2 = new DataView(data)
  view2.setUint32(2 + 36, headerBytes.byteLength)
  view2.setUint32(2 + 36 + 4 + headerBytes.byteLength, bodyLen)

  return view
}


// type: 21
// messageMeta
// version 1 byte + type 1 byte + content left bytes
export const buildMessageMeta = (messageType, rules) => {
  if (messageType !== sendMessageEnum.changeBreakPointRules) {
    throw new Error('invalid message type')
  }

  const rulesBytes = new TextEncoder().encode(JSON.stringify(rules))
  const view = new Uint8Array(2 + rulesBytes.byteLength)
  view[0] = 1
  view[1] = messageType
  view.set(rulesBytes, 2)

  return view
}

import type { IConnection } from './connection'
import type { Flow, IFlowRequest, IRequest, IResponse } from './flow'
import { delHeader, hasHeader, setHeader } from './utils'

const MESSAGE_VERSION = 2

export enum MessageType {
  CONN = 0,
  CONN_CLOSE = 5,
  REQUEST = 1,
  REQUEST_BODY = 2,
  RESPONSE = 3,
  RESPONSE_BODY = 4,
}

const allMessageBytes = [
  MessageType.CONN,
  MessageType.CONN_CLOSE,
  MessageType.REQUEST,
  MessageType.REQUEST_BODY,
  MessageType.RESPONSE,
  MessageType.RESPONSE_BODY,
]

export interface IMessage {
  type: MessageType
  id: string
  waitIntercept: boolean
  content?: ArrayBuffer | IFlowRequest | IResponse | IConnection | number
}

// type: 0/1/2/3/4
// messageFlow
// version 1 byte + type 1 byte + id 36 byte + waitIntercept 1 byte + content left bytes
export const parseMessage = (data: ArrayBuffer): IMessage | null => {
  if (data.byteLength < 39) return null
  const meta = new Int8Array(data.slice(0, 39))
  const version = meta[0]
  if (version !== MESSAGE_VERSION) return null
  const type = meta[1] as MessageType
  if (!allMessageBytes.includes(type)) return null
  const id = new TextDecoder().decode(data.slice(2, 38))
  const waitIntercept = meta[38] === 1

  const resp: IMessage = {
    type,
    id,
    waitIntercept,
  }
  if (data.byteLength === 39) return resp
  if (type === MessageType.REQUEST_BODY || type === MessageType.RESPONSE_BODY) {
    resp.content = data.slice(39)
    return resp
  }
  if (type === MessageType.CONN_CLOSE) {
    const view = new DataView(data.slice(39))
    resp.content = view.getUint32(0, false)
    return resp
  }

  const contentStr = new TextDecoder().decode(data.slice(39))
  let content: any
  try {
    content = JSON.parse(contentStr)
  } catch (err) {
    return null
  }

  resp.content = content
  return resp
}

export enum SendMessageType {
  CHANGE_REQUEST = 11,
  CHANGE_RESPONSE = 12,
  DROP_REQUEST = 13,
  DROP_RESPONSE = 14,
  CHANGE_BREAK_POINT_RULES = 21,
}

// type: 11/12/13/14
// messageEdit
// version 1 byte + type 1 byte + id 36 byte + header len 4 byte + header content bytes + body len 4 byte + [body content bytes]
export const buildMessageEdit = (messageType: SendMessageType, flow: Flow) => {
  if (messageType === SendMessageType.DROP_REQUEST || messageType === SendMessageType.DROP_RESPONSE) {
    const view = new Uint8Array(38)
    view[0] = MESSAGE_VERSION
    view[1] = messageType
    view.set(new TextEncoder().encode(flow.id), 2)
    return view
  }

  let header: Omit<IRequest, 'body'> | Omit<IResponse, 'body'>
  let body: ArrayBuffer | Uint8Array | undefined

  if (messageType === SendMessageType.CHANGE_REQUEST) {
    ({ body, ...header } = flow.request)
  } else if (messageType === SendMessageType.CHANGE_RESPONSE) {
    ({ body, ...header } = flow.response as IResponse)
  } else {
    throw new Error('invalid message type')
  }

  if (body instanceof ArrayBuffer) body = new Uint8Array(body)
  const bodyLen = (body && body.byteLength) ? body.byteLength : 0

  if (hasHeader(header.header, 'Content-Encoding')) delHeader(header.header, 'Content-Encoding')
  if (hasHeader(header.header, 'Transfer-Encoding')) delHeader(header.header, 'Transfer-Encoding')
  setHeader(header.header, 'Content-Length', [String(bodyLen)])

  const headerBytes = new TextEncoder().encode(JSON.stringify(header))
  const len = 2 + 36 + 4 + headerBytes.byteLength + 4 + bodyLen
  const data = new ArrayBuffer(len)
  const view = new Uint8Array(data)
  view[0] = MESSAGE_VERSION
  view[1] = messageType
  view.set(new TextEncoder().encode(flow.id), 2)
  view.set(headerBytes, 2 + 36 + 4)
  if (bodyLen) view.set(body as Uint8Array, 2 + 36 + 4 + headerBytes.byteLength + 4)

  const view2 = new DataView(data)
  view2.setUint32(2 + 36, headerBytes.byteLength)
  view2.setUint32(2 + 36 + 4 + headerBytes.byteLength, bodyLen)

  return view
}

// type: 21
// messageMeta
// version 1 byte + type 1 byte + content left bytes
export const buildMessageMeta = (messageType: SendMessageType, rules: any) => {
  if (messageType !== SendMessageType.CHANGE_BREAK_POINT_RULES) {
    throw new Error('invalid message type')
  }

  const rulesBytes = new TextEncoder().encode(JSON.stringify(rules))
  const view = new Uint8Array(2 + rulesBytes.byteLength)
  view[0] = MESSAGE_VERSION
  view[1] = messageType
  view.set(rulesBytes, 2)

  return view
}

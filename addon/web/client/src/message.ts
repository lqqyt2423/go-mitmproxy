import { getSize } from './utils'

export enum MessageType {
  REQUEST = 1,
  REQUEST_BODY = 2,
  RESPONSE = 3,
  RESPONSE_BODY = 4,
}

export type Header = Record<string, string[]>

export interface IRequest {
  method: string
  url: string
  proto: string
  header: Header
  body?: ArrayBuffer
}

export interface IResponse {
  statusCode: number
  header: Header
  body?: ArrayBuffer
}

export interface IMessage {
  type: MessageType
  id: string
  waitIntercept: boolean
  content?: ArrayBuffer | IRequest | IResponse
}

export interface IFlowPreview {
  no: number
  id: string
  waitIntercept: boolean
  host: string
  path: string
  method: string
  statusCode: string
  size: string
}

export class Flow {
  public no: number
  public id: string
  public waitIntercept: boolean
  public request: IRequest
  public response: IResponse | null = null

  private url: URL
  private _host = ''
  private _path = ''
  private _size = ''

  public static curNo = 0

  constructor(msg: IMessage) {
    this.no = ++Flow.curNo
    this.id = msg.id
    this.waitIntercept = msg.waitIntercept
    this.request = msg.content as IRequest

    this.url = new URL(this.request.url)
  }

  public addRequestBody(msg: IMessage): Flow {
    this.waitIntercept = msg.waitIntercept
    this.request.body = msg.content as ArrayBuffer
    return this
  }

  public addResponse(msg: IMessage): Flow {
    this.waitIntercept = msg.waitIntercept
    this.response = msg.content as IResponse
    return this
  }

  public addResponseBody(msg: IMessage): Flow {
    this.waitIntercept = msg.waitIntercept
    if (this.response) this.response.body = msg.content as ArrayBuffer
    return this
  }

  public preview(): IFlowPreview {
    return {
      no: this.no,
      id: this.id,
      waitIntercept: this.waitIntercept,
      host: this.host,
      path: this.path,
      method: this.request.method,
      statusCode: this.response ? String(this.response.statusCode) : '(pending)',
      size: this.size,
    }
  }

  private get host(): string {
    if (this._host) return this._host
    let _host = this.url.host
    if (_host.length > 35) _host = _host.slice(0, 35) + '...'
    this._host = _host
    return _host
  }

  private get path(): string {
    if (this._path) return this._path
    let _path = this.url.pathname + this.url.search
    if (_path.length > 65) _path = _path.slice(0, 65) + '...'
    this._path = _path
    return _path
  }

  private get size(): string {
    if (!this.response) return '0'
    if (!this.response.header) return '0'
    if (this._size) return this._size

    this._size = getSize(this.response)
    return this._size
  }
}

const allMessageBytes = [
  MessageType.REQUEST,
  MessageType.REQUEST_BODY,
  MessageType.RESPONSE,
  MessageType.RESPONSE_BODY,
]


// type: 1/2/3/4
// messageFlow
// version 1 byte + type 1 byte + id 36 byte + waitIntercept 1 byte + content left bytes
export const parseMessage = (data: ArrayBuffer): IMessage | null => {
  if (data.byteLength < 39) return null
  const meta = new Int8Array(data.slice(0, 39))
  const version = meta[0]
  if (version !== 1) return null
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
    view[0] = 1
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

  if ('Content-Encoding' in header.header) delete header.header['Content-Encoding']
  if ('Transfer-Encoding' in header.header) delete header.header['Transfer-Encoding']
  header.header['Content-Length'] = [String(bodyLen)]

  const headerBytes = new TextEncoder().encode(JSON.stringify(header))
  const len = 2 + 36 + 4 + headerBytes.byteLength + 4 + bodyLen
  const data = new ArrayBuffer(len)
  const view = new Uint8Array(data)
  view[0] = 1
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
  view[0] = 1
  view[1] = messageType
  view.set(rulesBytes, 2)

  return view
}

import { arrayBufferToBase64, bufHexView, getSize, isTextBody } from './utils'

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
  costTime: string
  contentType: string
}

interface IPreviewBody {
  type: 'image' | 'json' | 'binary'
  data: string | null
}

export class Flow {
  public no: number
  public id: string
  public waitIntercept: boolean
  public request: IRequest
  public response: IResponse | null = null

  public url: URL
  private path: string
  private _size = 0
  private size = '0'
  private headerContentLengthExist = false
  private contentType = ''

  private startTime = Date.now()
  private endTime = 0
  private costTime = '(pending)'

  public static curNo = 0

  private status: MessageType = MessageType.REQUEST

  private _isTextRequest: boolean | null
  private _isTextResponse: boolean | null
  private _requestBody: string | null
  private _hexviewRequestBody: string | null = null
  private _responseBody: string | null

  private _previewResponseBody: IPreviewBody | null = null
  private _previewRequestBody: IPreviewBody | null = null
  private _hexviewResponseBody: string | null = null

  constructor(msg: IMessage) {
    this.no = ++Flow.curNo
    this.id = msg.id
    this.waitIntercept = msg.waitIntercept
    this.request = msg.content as IRequest

    this.url = new URL(this.request.url)
    this.path = this.url.pathname + this.url.search

    this._isTextRequest = null
    this._isTextResponse = null
    this._requestBody = null
    this._responseBody = null
  }

  public addRequestBody(msg: IMessage): Flow {
    this.status = MessageType.REQUEST_BODY
    this.waitIntercept = msg.waitIntercept
    this.request.body = msg.content as ArrayBuffer
    return this
  }

  public addResponse(msg: IMessage): Flow {
    this.status = MessageType.RESPONSE
    this.waitIntercept = msg.waitIntercept
    this.response = msg.content as IResponse

    if (this.response && this.response.header) {
      if (this.response.header['Content-Type'] != null) {
        this.contentType = this.response.header['Content-Type'][0].split(';')[0]
        if (this.contentType.includes('javascript')) this.contentType = 'javascript'
      }
      if (this.response.header['Content-Length'] != null) {
        this.headerContentLengthExist = true
        this._size = parseInt(this.response.header['Content-Length'][0])
        this.size = getSize(this._size)
      }
    }

    return this
  }

  public addResponseBody(msg: IMessage): Flow {
    this.status = MessageType.RESPONSE_BODY
    this.waitIntercept = msg.waitIntercept
    if (this.response) this.response.body = msg.content as ArrayBuffer
    this.endTime = Date.now()
    this.costTime = String(this.endTime - this.startTime) + ' ms'

    if (!this.headerContentLengthExist && this.response && this.response.body) {
      this._size = this.response.body.byteLength
      this.size = getSize(this._size)
    }
    return this
  }

  public preview(): IFlowPreview {
    return {
      no: this.no,
      id: this.id,
      waitIntercept: this.waitIntercept,
      host: this.url.host,
      path: this.path,
      method: this.request.method,
      statusCode: this.response ? String(this.response.statusCode) : '(pending)',
      size: this.size,
      costTime: this.costTime,
      contentType: this.contentType,
    }
  }

  public isTextRequest(): boolean {
    if (this._isTextRequest !== null) return this._isTextRequest
    this._isTextRequest = isTextBody(this.request)
    return this._isTextRequest
  }

  public requestBody(): string {
    if (this._requestBody !== null) return this._requestBody
    if (!this.isTextRequest()) {
      this._requestBody = ''
      return this._requestBody
    }
    if (this.status < MessageType.REQUEST_BODY) return ''
    this._requestBody = new TextDecoder().decode(this.request.body)
    return this._requestBody
  }

  public hexviewRequestBody(): string | null {
    if (this._hexviewRequestBody !== null) return this._hexviewRequestBody
    if (this.status < MessageType.REQUEST_BODY) return null
    if (!(this.request?.body?.byteLength)) return null

    this._hexviewRequestBody = bufHexView(this.request.body)
    return this._hexviewRequestBody
  }

  public isTextResponse(): boolean | null {
    if (this.status < MessageType.RESPONSE) return null
    if (this._isTextResponse !== null) return this._isTextResponse
    this._isTextResponse = isTextBody(this.response as IResponse)
    return this._isTextResponse
  }

  public responseBody(): string {
    if (this._responseBody !== null) return this._responseBody
    if (this.status < MessageType.RESPONSE) return ''
    if (!this.isTextResponse()) {
      this._responseBody = ''
      return this._responseBody
    }
    if (this.status < MessageType.RESPONSE_BODY) return ''
    this._responseBody = new TextDecoder().decode(this.response?.body)
    return this._responseBody
  }

  public previewResponseBody(): IPreviewBody | null {
    if (this._previewResponseBody) return this._previewResponseBody

    if (this.status < MessageType.RESPONSE_BODY) return null
    if (!(this.response?.body?.byteLength)) return null

    let contentType: string | undefined
    if (this.response.header['Content-Type']) contentType = this.response.header['Content-Type'][0]
    if (!contentType) return null

    if (contentType.startsWith('image/')) {
      this._previewResponseBody = {
        type: 'image',
        data: arrayBufferToBase64(this.response.body),
      }
    }
    else if (contentType.includes('application/json')) {
      this._previewResponseBody = {
        type: 'json',
        data: this.responseBody(),
      }
    }

    return this._previewResponseBody
  }

  public previewRequestBody(): IPreviewBody | null {
    if (this._previewRequestBody) return this._previewRequestBody

    if (this.status < MessageType.REQUEST_BODY) return null
    if (!(this.request.body?.byteLength)) return null

    if (!this.isTextRequest()) {
      this._previewRequestBody = {
        type: 'binary',
        data: this.hexviewRequestBody(),
      }
    } else if (/json/.test(this.request.header['Content-Type'].join(''))) {
      this._previewRequestBody = {
        type: 'json',
        data: this.requestBody(),
      }
    }

    return this._previewRequestBody
  }

  public hexviewResponseBody(): string | null {
    if (this._hexviewResponseBody !== null) return this._hexviewResponseBody

    if (this.status < MessageType.RESPONSE_BODY) return null
    if (!(this.response?.body?.byteLength)) return null

    this._hexviewResponseBody = bufHexView(this.response.body)
    return this._hexviewResponseBody
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

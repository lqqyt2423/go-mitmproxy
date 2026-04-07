import type { ConnectionManager, IConnection } from './connection'
import { IMessage, MessageType, IWebSocketMessage, IWebSocketMessageData, ISSEEvent, ISSEMessageData } from './message'
import { arrayBufferToBase64, bufHexView, getHeader, getSize, hasHeader, isTextBody } from './utils'
import { FlowFilter } from './filter'

export type Header = Record<string, string[]>

export interface IRequest {
  method: string
  url: string
  proto: string
  header: Header
  body?: ArrayBuffer
}

export interface IFlowRequest {
  connId: string
  request: IRequest
}

export interface IResponse {
  statusCode: number
  header: Header
  body?: ArrayBuffer
}

export interface IPreviewBody {
  type: 'image' | 'json' | 'binary' | 'x-json-stream'
  data: string | null
}

export interface IFlowAnnotation {
  color: string
  comment: string
}

export interface ITimingData {
  dnsMs: number
  connectMs: number
  tlsMs: number
  sendMs: number
  waitMs: number
  receiveMs: number
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
  warn: boolean
}

export class Flow {
  public no: number
  public id: string
  public connId!: string
  public waitIntercept!: boolean
  public request!: IRequest
  public response: IResponse | null = null

  public url!: URL
  private path!: string
  private _size = 0
  private size = '0'
  private headerContentLengthExist = false
  private contentType = ''

  private startTime = Date.now()
  private endTime = 0
  private costTime = '(pending)'

  public static curNo = 0

  private status: MessageType = MessageType.REQUEST

  private _requestBody: string | null
  private _isTextRequest: boolean | null
  private _previewRequestBody: IPreviewBody | null = null
  private _hexviewRequestBody: string | null = null

  private _responseBody: string | null
  private _isTextResponse: boolean | null
  private _previewResponseBody: IPreviewBody | null = null
  private _hexviewResponseBody: string | null = null

  // WebSocket 相关字段
  public webSocketMessages: IWebSocketMessage[] = []
  public isWebSocket = false

  // SSE 相关字段
  public sseEvents: ISSEEvent[] = []
  public isSSE = false

  // Annotation (highlight + comment)
  public annotation: IFlowAnnotation | null = null
  // Timing data
  public timing: ITimingData | null = null

  // AI API 类型 (null 表示不是 AI API)
  public aiAPIType: 'anthropic' | 'openai' | null = null

  private connMgr: ConnectionManager
  private conn: IConnection | undefined

  constructor(msg: IMessage, connMgr: ConnectionManager) {
    this.no = ++Flow.curNo
    this.id = msg.id

    this.addRequest(msg)

    this._isTextRequest = null
    this._isTextResponse = null
    this._requestBody = null
    this._responseBody = null

    this.connMgr = connMgr
  }

  public addRequest(msg: IMessage): Flow {
    this.status = MessageType.REQUEST
    this.waitIntercept = msg.waitIntercept

    const flowRequestMsg = msg.content as IFlowRequest
    this.connId = flowRequestMsg.connId
    this.request = flowRequestMsg.request

    let rawUrl = this.request.url
    if (rawUrl.startsWith('//')) rawUrl = 'http:' + rawUrl

    // 处理 WebSocket URL（可能是相对路径，如 "/ws"）
    if (rawUrl.startsWith('/')) {
      // 尝试从 Host header 获取主机名
      let host = 'localhost'
      if (this.request.header && this.request.header['Host']) {
        host = this.request.header['Host'][0]
      }
      // 对于 WebSocket，使用 ws:// 协议
      rawUrl = 'ws://' + host + rawUrl
    }

    try {
      this.url = new URL(rawUrl)
    } catch (e) {
      // 如果仍然失败，使用一个默认 URL
      console.error('Failed to parse URL:', rawUrl, e)
      this.url = new URL('http://localhost/')
    }

    this.path = this.url.pathname + this.url.search

    return this
  }

  public addRequestBody(msg: IMessage): Flow {
    this.status = MessageType.REQUEST_BODY
    this.waitIntercept = msg.waitIntercept
    this.request.body = msg.content as ArrayBuffer
    this._requestBody = null
    this._isTextRequest = null
    this._previewRequestBody = null
    this._hexviewRequestBody = null
    return this
  }

  public addResponse(msg: IMessage): Flow {
    this.status = MessageType.RESPONSE
    this.waitIntercept = msg.waitIntercept
    this.response = msg.content as IResponse

    if (this.response && this.response.header) {
      if (hasHeader(this.response.header, 'Content-Type')) {
        this.contentType = getHeader(this.response.header, 'Content-Type')[0].split(';')[0]
        if (this.contentType.includes('javascript')) this.contentType = 'javascript'
      }
      if (hasHeader(this.response.header, 'Content-Length')) {
        this.headerContentLengthExist = true
        this._size = parseInt(getHeader(this.response.header, 'Content-Length')[0])
        this.size = getSize(this._size)
      }
    }

    return this
  }

  public addResponseBody(msg: IMessage): Flow {
    this.status = MessageType.RESPONSE_BODY
    this.waitIntercept = msg.waitIntercept
    if (this.response) {
      this.response.body = msg.content as ArrayBuffer
      this._responseBody = null
      this._isTextResponse = null
      this._previewResponseBody = null
      this._hexviewResponseBody = null
    }
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
      warn: this.getConn()?.flowCount === 0,
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
    if (hasHeader(this.response.header, 'Content-Type')) contentType = getHeader(this.response.header, 'Content-Type')[0]
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
    } else if (getHeader(this.request.header, 'Content-Type').join('').toLowerCase().includes('x-json-stream')) {
      this._previewRequestBody = {
        type: 'x-json-stream',
        data: this.requestBody(),
      }
    } else if (/json/.test(getHeader(this.request.header, 'Content-Type').join(''))) {
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

  // WebSocket 相关方法
  public addWebSocketMessage(msg: IMessage): Flow {
    const wsMsgData = msg.content as IWebSocketMessageData
    this.isWebSocket = true
    this.webSocketMessages.push(wsMsgData.message)
    return this
  }

  public setWebSocketStart(): Flow {
    this.isWebSocket = true
    return this
  }

  // SSE 相关方法
  public addSSEMessage(msg: IMessage): Flow {
    const sseMsgData = msg.content as ISSEMessageData
    this.isSSE = true
    this.sseEvents.push(sseMsgData.event)
    return this
  }

  public setSSEStart(): Flow {
    this.isSSE = true
    return this
  }

  public getConn(): IConnection | undefined {
    if (this.conn) return this.conn
    this.conn = this.connMgr.get(this.connId)
    return this.conn
  }

  public detectAIAPI(): void {
    this.aiAPIType = null

    const reqContentType = getHeader(this.request.header, 'Content-Type')?.[0]
    const resContentType = this.response?.header && getHeader(this.response.header, 'Content-Type')?.[0]

    if (!reqContentType?.includes('application/json') || !resContentType?.includes('text/event-stream')) {
      return
    }

    try {
      const reqBody = JSON.parse(this.requestBody())
      if (!reqBody.model || typeof reqBody.model !== 'string' || !Array.isArray(reqBody.messages) || reqBody.stream !== true) {
        return
      }

      // 检测 system role (OpenAI 格式)
      if (reqBody.messages.some((msg: any) => msg.role === 'system')) {
        this.aiAPIType = 'openai'
        return
      }

      // 检测 system 字段 (Anthropic 格式)
      if (reqBody.system) {
        this.aiAPIType = 'anthropic'
        return
      }

      // 通过 URL 检测
      const url = this.request.url.toLowerCase()
      if (url.includes('anthropic')) {
        this.aiAPIType = 'anthropic'
        return
      }
      if (url.includes('chat/completions')) {
        this.aiAPIType = 'openai'
        return
      }

      // 通过 Header 检测
      if (Object.keys(this.request.header).some((key) => key.toLowerCase().includes('anthropic'))) {
        this.aiAPIType = 'anthropic'
        return
      }

      const userAgent = getHeader(this.request.header, 'User-Agent')?.[0]?.toLowerCase()
      if (userAgent?.includes('openai')) {
        this.aiAPIType = 'openai'
        return
      }
    } catch {
      // JSON 解析失败，忽略
    }
  }
}

export class FlowManager {
  private items: Flow[]
  private _map: Map<string, Flow>
  private flowFilter: FlowFilter | undefined
  private filterTimer: number | null
  private num: number
  private max: number

  constructor() {
    this.items = []
    this._map = new Map()
    this.filterTimer = null
    this.num = 0

    this.max = 1000
  }

  showList() {
    if (!this.flowFilter) return this.items
    return this.items.filter(item => (this.flowFilter as FlowFilter).match(item))
  }

  add(item: Flow) {
    item.no = ++this.num
    this.items.push(item)
    this._map.set(item.id, item)

    if (this.items.length > this.max) {
      const oldest = this.items.shift()
      if (oldest) this._map.delete(oldest.id)
    }
  }

  get(id: string) {
    return this._map.get(id)
  }

  changeFilterLazy(text: string, callback: (err: any) => void) {
    if (this.filterTimer) {
      clearTimeout(this.filterTimer)
      this.filterTimer = null
    }

    this.filterTimer = setTimeout(() => {
      try {
        this.flowFilter = new FlowFilter(text)
        callback(null)
      } catch (err) {
        this.flowFilter = undefined
        callback(err)
      }
    }, 300) as any
  }

  clear() {
    this.items = []
    this._map = new Map()
  }
}

import React, { useState, useEffect } from 'react'
import Button from 'react-bootstrap/Button'
import FormCheck from 'react-bootstrap/FormCheck'
import Table from 'react-bootstrap/Table'
import Badge from 'react-bootstrap/Badge'
import fetchToCurl from 'fetch-to-curl'
import copy from 'copy-to-clipboard'
import JSONPretty from 'react-json-pretty'
import { flattenHeader, isTextBody } from '../utils/utils'
import type { Flow, IResponse } from '../utils/flow'
import EditFlow from './EditFlow'
import { useSize } from 'ahooks'
import { ResizerItem } from '../components/ResizerItem'
import { configViewFlowRequestBodyTab, configViewFlowResponseBodyLineBreak, configViewFlowTab, useConfig } from '../utils/config'

interface Iprops {
  flow: Flow | null
  onClose: () => void
  onReRenderFlows: () => void
  onMessage: (msg: ArrayBufferLike) => void
}

function ViewFlow({ flow, onClose, onReRenderFlows, onMessage }: Iprops) {
  const bodySize = useSize(document.querySelector('body'))
  const initWrapWidth = bodySize ? bodySize.width / 2 : 500
  const maxWrapWidth = bodySize ? bodySize.width * 0.9 : 1000
  const minWrapWidth = 500
  const [wrapWidth, setWrapWidth] = useState(initWrapWidth)

  const [flowTab, setFlowTab] = useConfig(configViewFlowTab)
  const [copied, setCopied] = useState(false)
  const [requestBodyViewTab, setRequestBodyViewTab] = useConfig(configViewFlowRequestBodyTab)
  const [responseBodyLineBreak, setResponseBodyLineBreak] = useConfig(configViewFlowResponseBodyLineBreak)

  // 当 Flow 不是 WebSocket 但当前 tab 是 WebSocket 时，自动切换到 Detail
  useEffect(() => {
    if (flow && flowTab === 'WebSocket' && !flow.isWebSocket) {
      setFlowTab('Detail')
    }
    if (flow && flowTab === 'SSE' && !flow.isSSE) {
      setFlowTab('Detail')
    }
  }, [flow, flowTab, setFlowTab])

  const copyAsCurl = () => {
    if (!flow) return null
    return (
      <Button size="sm" variant={copied ? 'success' : 'primary'} disabled={copied} onClick={() => {
        const curl = fetchToCurl({
          url: flow.request.url,
          method: flow.request.method,
          headers: Object.keys(flow.request.header).reduce((obj: any, key: string) => {
            obj[key] = flow.request.header[key][0]
            return obj
          }, {}),
          body: flow.requestBody(),
        })
        copy(curl)
        setCopied(true)
        setTimeout(() => {
          setCopied(false)
        }, 1000)
      }}>{copied ? 'Copied' : 'Copy as cURL'}</Button>
    )
  }

  const preview = () => {
    if (!flow) return null
    const response = flow.response
    if (!response) return null

    if (!(response.body && response.body.byteLength)) {
      return <div style={{ color: 'gray' }}>No response</div>
    }

    const pv = flow.previewResponseBody()
    if (!pv) return <div style={{ color: 'gray' }}>Not support preview</div>

    if (pv.type === 'image') {
      return <img src={`data:image/png;base64,${pv.data}`} />
    }
    else if (pv.type === 'json') {
      return <div><JSONPretty data={pv.data} keyStyle={'color: rgb(130,40,144);'} stringStyle={'color: rgb(153,68,60);'} valueStyle={'color: rgb(25,1,199);'} booleanStyle={'color: rgb(94,105,192);'} /></div>
    }

    return <div style={{ color: 'gray' }}>Not support preview</div>
  }

  const requestBodyPreview = () => {
    if (!flow) return null

    const pv = flow.previewRequestBody()
    if (!pv) return <div style={{ color: 'gray' }}>Not support preview</div>

    if (pv.type === 'json') {
      return <div><JSONPretty data={pv.data} keyStyle={'color: rgb(130,40,144);'} stringStyle={'color: rgb(153,68,60);'} valueStyle={'color: rgb(25,1,199);'} booleanStyle={'color: rgb(94,105,192);'} /></div>
    }
    else if (pv.type === 'x-json-stream') {
      return <div>
        {
          pv.data?.split('\n').map((line, index) => {
            if (!line) return null

            return <JSONPretty key={flow.id + index} data={line} keyStyle={'color: rgb(130,40,144);'} stringStyle={'color: rgb(153,68,60);'} valueStyle={'color: rgb(25,1,199);'} booleanStyle={'color: rgb(94,105,192);'} />
          })
        }
      </div>
    }
    else if (pv.type === 'binary') {
      return <div><pre>{pv.data}</pre></div>
    }

    return <div style={{ color: 'gray' }}>Not support preview</div>
  }

  const hexview = () => {
    if (!flow) return null
    const response = flow.response
    if (!response) return null

    if (!(response.body && response.body.byteLength)) {
      return <div style={{ color: 'gray' }}>No response</div>
    }

    return <pre>{flow.hexviewResponseBody()}</pre>
  }

  const websocketView = () => {
    if (!flow) return null
    if (!flow.isWebSocket) {
      return <div style={{ color: 'gray' }}>Not a WebSocket connection</div>
    }

    if (flow.webSocketMessages.length === 0) {
      return <div style={{ color: 'gray' }}>No WebSocket messages yet</div>
    }

    // 尝试解码 base64 内容
    const decodeContent = (content: string): string => {
      try {
        const decoded = atob(content)
        // 检查是否是可打印的文本
        if (/^[\x20-\x7E\r\n\t]*$/.test(decoded)) {
          return decoded
        }
        return content // 保留 base64 格式
      } catch {
        return content
      }
    }

    return (
      <Table striped bordered hover size="sm">
        <thead>
          <tr>
            <th style={{ width: '60px' }}>Index</th>
            <th style={{ width: '80px' }}>Direction</th>
            <th style={{ width: '60px' }}>Type</th>
            <th>Content</th>
            <th style={{ width: '180px' }}>Time</th>
          </tr>
        </thead>
        <tbody>
          {flow.webSocketMessages.map((msg, index) => (
            <tr key={index}>
              <td>{index}</td>
              <td>
                {msg.fromClient ?
                  <Badge bg="primary">C → S</Badge> :
                  <Badge bg="success">S → C</Badge>
                }
              </td>
              <td>
                {msg.type === 1 ?
                  <Badge bg="info">Text</Badge> :
                  <Badge bg="secondary">Binary</Badge>
                }
              </td>
              <td>
                <div style={{
                  maxWidth: '500px',
                  wordBreak: 'break-all',
                  whiteSpace: 'pre-wrap',
                  fontSize: '12px'
                }}>
                  {decodeContent(msg.content)}
                </div>
              </td>
              <td style={{ fontSize: '11px', color: '#666' }}>
                {new Date(msg.timestamp).toLocaleTimeString()}
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
    )
  }

  const sseView = () => {
    if (!flow) return null
    if (!flow.isSSE) {
      return <div style={{ color: 'gray' }}>Not a SSE connection</div>
    }

    if (flow.sseEvents.length === 0) {
      return <div style={{ color: 'gray' }}>No SSE events yet</div>
    }

    return (
      <Table striped bordered hover size="sm">
        <thead>
          <tr>
            <th style={{ width: '60px' }}>Index</th>
            <th style={{ width: '80px' }}>Event</th>
            <th style={{ width: '60px' }}>ID</th>
            <th>Data</th>
            <th style={{ width: '180px' }}>Time</th>
          </tr>
        </thead>
        <tbody>
          {flow.sseEvents.map((event, index) => (
            <tr key={index}>
              <td>{index}</td>
              <td>
                {event.event ?
                  <Badge bg="info">{event.event}</Badge> :
                  <Badge bg="secondary">message</Badge>
                }
              </td>
              <td>
                {event.id ?
                  <Badge bg="primary">{event.id}</Badge> :
                  <span style={{ color: '#999' }}>N/A</span>
                }
              </td>
              <td>
                <div style={{
                  maxWidth: '500px',
                  wordBreak: 'break-all',
                  whiteSpace: 'pre-wrap',
                  fontSize: '12px'
                }}>
                  {event.data}
                </div>
              </td>
              <td style={{ fontSize: '11px', color: '#666' }}>
                {new Date(event.timestamp).toLocaleTimeString()}
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
    )
  }

  const detail = () => {
    if (!flow) return null

    const conn = flow.getConn()

    return (
      <div>
        <div className="header-block">
          <p>Flow Info</p>
          <div className="header-block-content">
            <p>Id: {flow.id}</p>
          </div>
        </div>
        {
          !conn ? null :
            <>
              {
                !conn.serverConn ? null :
                  <>
                    <div className="header-block">
                      <p>Server Connection</p>
                      <div className="header-block-content">
                        <p>Address: {conn.serverConn.address}</p>
                        <p>Resolved Address: {conn.serverConn.peername}</p>
                      </div>
                    </div>
                  </>
              }
              <div className="header-block">
                <p>Client Connection</p>
                <div className="header-block-content">
                  <p>Address: {conn.clientConn.address}</p>
                </div>
              </div>
              <div className="header-block">
                <p>Connection Info</p>
                <div className="header-block-content">
                  <p>Id: {conn.clientConn.id}</p>
                  <p>Intercept: {conn.intercept ? 'true' : 'false'}</p>
                  {
                    conn.opening == null ? null :
                      <p>Opening: {conn.opening ? 'true' : 'false'}</p>
                  }
                  {
                    conn.flowCount == null ? null :
                      <p>Flow Count: {conn.flowCount}</p>
                  }
                </div>
              </div>
            </>
        }
      </div>
    )
  }

  if (!flow) return null

  const request = flow.request
  const response: IResponse = (flow.response || {}) as any

  // Query String Parameters
  const searchItems: Array<{ key: string; value: string }> = []
  if (flow.url && flow.url.search) {
    flow.url.searchParams.forEach((value, key) => {
      searchItems.push({ key, value })
    })
  }

  return (
    <div className="flow-detail" style={{ width: wrapWidth }}>
      <ResizerItem
        width={wrapWidth}
        setWidth={setWrapWidth}
        left={0}
        minWidth={minWrapWidth}
        maxWidth={maxWrapWidth}
      />

      <div className="header-tabs">
        <span
          style={{
            position: 'absolute',
            top: '2px',
            left: '0px',
            cursor: 'pointer',
          }}
          onClick={() => { onClose() }}>x</span>

        <EditFlow
          flow={flow}
          onChangeRequest={request => {
            flow.request.method = request.method
            flow.request.url = request.url
            flow.request.header = request.header
            if (isTextBody(flow.request)) flow.request.body = request.body
            onReRenderFlows()
          }}
          onChangeResponse={response => {
            if (!flow.response) flow.response = {} as IResponse

            flow.response.statusCode = response.statusCode
            flow.response.header = response.header
            if (isTextBody(flow.response)) flow.response.body = response.body
            onReRenderFlows()
          }}
          onMessage={msg => {
            onMessage(msg)
            flow.waitIntercept = false
            onReRenderFlows()
          }}
        />

        <div>{copyAsCurl()}</div>

        <div>
          <span className={flowTab === 'Detail' ? 'selected' : undefined} onClick={() => { setFlowTab('Detail') }}>Detail</span>
          <span className={flowTab === 'Headers' ? 'selected' : undefined} onClick={() => { setFlowTab('Headers') }}>Headers</span>
          <span className={flowTab === 'Preview' ? 'selected' : undefined} onClick={() => { setFlowTab('Preview') }}>Preview</span>
          <span className={flowTab === 'Response' ? 'selected' : undefined} onClick={() => { setFlowTab('Response') }}>Response</span>
          <span className={flowTab === 'Hexview' ? 'selected' : undefined} onClick={() => { setFlowTab('Hexview') }}>Hexview</span>
          {flow.isWebSocket && <span className={flowTab === 'WebSocket' ? 'selected' : undefined} onClick={() => { setFlowTab('WebSocket') }}>WebSocket</span>}
          {flow.isSSE && <span className={flowTab === 'SSE' ? 'selected' : undefined} onClick={() => { setFlowTab('SSE') }}>SSE</span>}
        </div>
      </div>

      <div style={{ padding: '20px 25px' }}>
        {
          !(flowTab === 'Headers') ? null :
            <div>
              <div className="header-block">
                <p>General</p>
                <div className="header-block-content">
                  <p>Request URL: {request.url}</p>
                  <p>Request Method: {request.method}</p>
                  <p>Status Code: {`${response.statusCode || '(pending)'}`}</p>
                </div>
              </div>

              {
                !(response.header) ? null :
                  <div className="header-block">
                    <p>Response Headers</p>
                    <div className="header-block-content">
                      {
                        flattenHeader(response.header).map(({ key, value }, index) => {
                          return (
                            <p key={flow.id + key + index}>{key}: {value}</p>
                          )
                        })
                      }
                    </div>
                  </div>
              }

              <div className="header-block">
                <p>Request Headers</p>
                <div className="header-block-content">
                  {
                    !(request.header) ? null :
                      flattenHeader(request.header).map(({ key, value }, index) => {
                        return (
                          <p key={flow.id + key + index}>{key}: {value}</p>
                        )
                      })
                  }
                </div>
              </div>

              {
                !(searchItems.length) ? null :
                  <div className="header-block">
                    <p>Query String Parameters</p>
                    <div className="header-block-content">
                      {
                        searchItems.map(({ key, value }, index) => {
                          return (
                            <p key={`${key}-${index}`}>{key}: {value}</p>
                          )
                        })
                      }
                    </div>
                  </div>
              }

              {
                !(request.body && request.body.byteLength) ? null :
                  <div className="header-block">
                    <p>Request Body</p>
                    <div className="header-block-content">
                      <div>
                        <div className="request-body-detail" style={{ marginBottom: '15px' }}>
                          <span className={requestBodyViewTab === 'Raw' ? 'selected' : undefined} onClick={() => { setRequestBodyViewTab('Raw') }}>Raw</span>
                          <span className={requestBodyViewTab === 'Preview' ? 'selected' : undefined} onClick={() => { setRequestBodyViewTab('Preview') }}>Preview</span>
                        </div>

                        {
                          !(requestBodyViewTab === 'Raw') ? null :
                            <div>
                              {
                                !(flow.isTextRequest()) ? <span style={{ color: 'gray' }}>Not text Request</span> : flow.requestBody()
                              }
                            </div>
                        }

                        {
                          !(requestBodyViewTab === 'Preview') ? null :
                            <div>{requestBodyPreview()}</div>
                        }
                      </div>
                    </div>
                  </div>
              }

            </div>
        }

        {
          !(flowTab === 'Response') ? null :
            !(response.body && response.body.byteLength) ? <div style={{ color: 'gray' }}>No response</div> :
              !(flow.isTextResponse()) ? <div style={{ color: 'gray' }}>Not text response</div> :
                <div>
                  <div style={{ marginBottom: '20px' }}>
                    <FormCheck
                      inline
                      type="checkbox"
                      checked={responseBodyLineBreak}
                      onChange={e => {
                        setResponseBodyLineBreak(e.target.checked)
                      }}
                      label="自动换行"></FormCheck>
                  </div>
                  <div style={{ whiteSpace: responseBodyLineBreak ? 'pre-wrap' : 'pre' }}>
                    {flow.responseBody()}
                  </div>
                </div>
        }

        {
          !(flowTab === 'Preview') ? null :
            <div>{preview()}</div>
        }

        {
          !(flowTab === 'WebSocket') ? null :
            <div>{websocketView()}</div>
        }

        {
          !(flowTab === 'SSE') ? null :
            <div>{sseView()}</div>
        }

        {
          !(flowTab === 'Hexview') ? null :
            <div>{hexview()}</div>
        }

        {
          !(flowTab === 'Detail') ? null :
            <div>{detail()}</div>
        }
      </div>

    </div>
  )
}

export default ViewFlow

import React, { useState } from 'react'
import Button from 'react-bootstrap/Button'
import FormCheck from 'react-bootstrap/FormCheck'
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
                            <p key={flow.id+key+index}>{key}: {value}</p>
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
                          <p key={flow.id+key+index}>{key}: {value}</p>
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

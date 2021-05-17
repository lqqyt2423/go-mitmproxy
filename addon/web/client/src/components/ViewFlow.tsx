import React from 'react'
import JSONPretty from 'react-json-pretty'
import { Flow, IResponse } from '../message'
import { isTextBody } from '../utils'

import EditFlow from './EditFlow'

interface Iprops {
  flow: Flow | null
  onClose: () => void
  onReRenderFlows: () => void
  onMessage: (msg: ArrayBufferLike) => void
}

interface IState {
  flowTab: 'Headers' | 'Preview' | 'Response'
}

class ViewFlow extends React.Component<Iprops, IState> {
  constructor(props: Iprops) {
    super(props)

    this.state = {
      flowTab: 'Headers',
    }
  }

  preview() {
    const { flow } = this.props
    if (!flow) return null
    const response = flow.response
    if(!response) return null

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

  render() {
    if (!this.props.flow) return null

    const flow = this.props.flow
    const flowTab = this.state.flowTab

    const request = flow.request
    const response: IResponse = (flow.response || {}) as any

    return (
      <div className="flow-detail">
        <div className="header-tabs">
          <span onClick={() => { this.props.onClose() }}>x</span>
          <span className={flowTab === 'Headers' ? 'selected' : undefined} onClick={() => { this.setState({ flowTab: 'Headers' }) }}>Headers</span>
          <span className={flowTab === 'Preview' ? 'selected' : undefined} onClick={() => { this.setState({ flowTab: 'Preview' }) }}>Preview</span>
          <span className={flowTab === 'Response' ? 'selected' : undefined} onClick={() => { this.setState({ flowTab: 'Response' }) }}>Response</span>

          <EditFlow
            flow={flow}
            onChangeRequest={request => {
              flow.request.method = request.method
              flow.request.url = request.url
              flow.request.header = request.header
              if (isTextBody(flow.request)) flow.request.body = request.body
              this.props.onReRenderFlows()
            }}
            onChangeResponse={response => {
              if (!flow.response) flow.response = {} as IResponse

              flow.response.statusCode = response.statusCode
              flow.response.header = response.header
              if (isTextBody(flow.response)) flow.response.body = response.body
              this.props.onReRenderFlows()
            }}
            onMessage={msg => {
              this.props.onMessage(msg)
              flow.waitIntercept = false
              this.props.onReRenderFlows()
            }}
          />

        </div>

        <div style={{ padding: '20px' }}>
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
                          Object.keys(response.header).map(key => {
                            return (
                              <p key={key}>{key}: {response.header[key].join(' ')}</p>
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
                        Object.keys(request.header).map(key => {
                          return (
                            <p key={key}>{key}: {request.header[key].join(' ')}</p>
                          )
                        })
                    }
                  </div>
                </div>

                {
                  !(request.body && request.body.byteLength) ? null :
                    <div className="header-block">
                      <p>Request Body</p>
                      <div className="header-block-content">
                        <p>
                          {
                            !(flow.isTextRequest()) ? <span style={{ color: 'gray' }}>Not text</span> :
                              flow.requestBody()
                          }
                        </p>
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
                    {flow.responseBody()}
                  </div>
          }

          {
            !(flowTab === 'Preview') ? null :
              <div>{this.preview()}</div>
          }
        </div>

      </div>
    )
  }
}

export default ViewFlow

import React from 'react'
import Table from 'react-bootstrap/Table'
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import './App.css'

import BreakPoint from './components/BreakPoint'
import EditFlow from './components/EditFlow'
import FlowPreview from './components/FlowPreview'

import { FlowManager } from './flow'
import { isTextBody } from './utils'
import { parseMessage, SendMessageType, buildMessageMeta, Flow, MessageType, IResponse } from './message'

interface IState {
  flows: Flow[]
  flow: Flow | null
  flowTab: 'Headers' | 'Preview' | 'Response'
}

class App extends React.Component<any, IState> {
  private flowMgr: FlowManager
  private ws: WebSocket | null

  constructor(props: any) {
    super(props)

    this.flowMgr = new FlowManager()

    this.state = {
      flows: this.flowMgr.showList(),
      flow: null,

      flowTab: 'Headers', // Headers, Preview, Response
    }

    this.ws = null
  }

  componentDidMount() {
    this.initWs()
  }

  componentWillUnmount() {
    if (this.ws) {
      this.ws.close()
    }
  }

  initWs() {
    if (this.ws) return

    let host
    if (process.env.NODE_ENV === 'development') {
      host = 'localhost:9081'
    } else {
      host = new URL(document.URL).host
    }
    this.ws = new WebSocket(`ws://${host}/echo`)
    this.ws.binaryType = 'arraybuffer'
    this.ws.onopen = () => { console.log('OPEN') }
    this.ws.onclose = () => { console.log('CLOSE') }
    this.ws.onmessage = evt => {
      const msg = parseMessage(evt.data)
      if (!msg) {
        console.error('parse error:', evt.data)
        return
      }
      // console.log('msg:', msg)

      if (msg.type === MessageType.REQUEST) {
        const flow = new Flow(msg)
        this.flowMgr.add(flow)
        this.setState({ flows: this.flowMgr.showList() })
      }
      else if (msg.type === MessageType.REQUEST_BODY) {
        const flow = this.flowMgr.get(msg.id)
        if (!flow) return
        flow.addRequestBody(msg)
        this.setState({ flows: this.state.flows })
      }
      else if (msg.type === MessageType.RESPONSE) {
        const flow = this.flowMgr.get(msg.id)
        if (!flow) return
        flow.addResponse(msg)
        this.setState({ flows: this.state.flows })
      }
      else if (msg.type === MessageType.RESPONSE_BODY) {
        const flow = this.flowMgr.get(msg.id)
        if (!flow || !flow.response) return
        flow.addResponseBody(msg)
        this.setState({ flows: this.state.flows })
      }
    }
    this.ws.onerror = evt => {
      console.log('ERROR:', evt)
    }
  }

  renderFlow() {
    const { flow, flowTab } = this.state
    if (!flow) return null

    const request = flow.request
    const response: IResponse = (flow.response || {}) as any

    return (
      <div className="flow-detail">
        <div className="header-tabs">
          <span onClick={() => { this.setState({ flow: null }) }}>x</span>
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
              this.setState({ flows: this.state.flows })
            }}
            onChangeResponse={response => {
              if (!flow.response) flow.response = {} as IResponse

              flow.response.statusCode = response.statusCode
              flow.response.header = response.header
              if (isTextBody(flow.response)) flow.response.body = response.body
              this.setState({ flows: this.state.flows })
            }}
            onMessage={msg => {
              if (this.ws) this.ws.send(msg)
              flow.waitIntercept = false
              this.setState({ flows: this.state.flows })
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
                            !(isTextBody(request)) ? 'Not text' :
                              new TextDecoder().decode(request.body)
                          }
                        </p>
                      </div>
                    </div>
                }

              </div>
          }

          {
            !(flowTab === 'Response') ? null :
              !(response.body && response.body.byteLength) ? <div>No response</div> :
                !(isTextBody(response)) ? <div>Not text response</div> :
                  <div>
                    {new TextDecoder().decode(response.body)}
                  </div>
          }
        </div>

      </div>
    )
  }

  render() {
    const { flows } = this.state
    return (
      <div className="main-table-wrap">
        <div className="top-control">
          <div><Button size="sm" onClick={() => {
            this.flowMgr.clear()
            this.setState({ flows: this.flowMgr.showList(), flow: null })
          }}>Clear</Button></div>
          <div>
            <Form.Control
              size="sm" placeholder="Filter"
              onChange={(e) => {
                const value = e.target.value
                this.flowMgr.changeFilterLazy(value, () => {
                  this.setState({ flows: this.flowMgr.showList() })
                })
              }}
            >
            </Form.Control>
          </div>

          <BreakPoint onSave={rules => {
            const msg = buildMessageMeta(SendMessageType.CHANGE_BREAK_POINT_RULES, rules)
            if (this.ws) this.ws.send(msg)
          }} />
        </div>

        <Table striped bordered size="sm" style={{ tableLayout: 'fixed' }}>
          <thead>
            <tr>
              <th style={{ width: '50px' }}>No</th>
              <th style={{ width: '200px' }}>Host</th>
              <th style={{ width: '500px' }}>Path</th>
              <th style={{ width: '80px' }}>Method</th>
              <th style={{ width: '80px' }}>Status</th>
              <th style={{ width: '90px' }}>Size</th>
              <th style={{ width: '90px' }}>Time</th>
            </tr>
          </thead>
          <tbody>
            {
              flows.map(f => {
                const fp = f.preview()

                return (
                  <FlowPreview
                    key={fp.id}
                    flow={fp}
                    isSelected={(this.state.flow && this.state.flow.id === fp.id) ? true : false}
                    onShowDetail={() => {
                      this.setState({ flow: f })
                    }}
                  />
                )
              })
            }
          </tbody>
        </Table>

        {this.renderFlow()}
      </div>
    )
  }
}

export default App

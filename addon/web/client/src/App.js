import React from 'react'
import Table from 'react-bootstrap/Table'
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import './App.css'

import BreakPoint from './components/BreakPoint'

import { FlowManager } from './flow'
import { isTextResponse, getSize } from './utils'
import { parseMessage, sendMessageEnum, buildMessageEdit, buildMessageMeta } from './message'

class App extends React.Component {

  constructor(props) {
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

      if (msg.type === 'request') {
        const flow = { id: msg.id, request: msg.content, waitIntercept: msg.waitIntercept }
        this.flowMgr.add(flow)
        this.setState({ flows: this.flowMgr.showList() })
      }
      else if (msg.type === 'requestBody') {
        const flow = this.flowMgr.get(msg.id)
        if (!flow) return
        flow.waitIntercept = msg.waitIntercept
        flow.request.body = msg.content
        this.setState({ flows: this.state.flows })
      }
      else if (msg.type === 'response') {
        const flow = this.flowMgr.get(msg.id)
        if (!flow) return
        flow.waitIntercept = msg.waitIntercept
        flow.response = msg.content
        this.setState({ flows: this.state.flows })
      }
      else if (msg.type === 'responseBody') {
        const flow = this.flowMgr.get(msg.id)
        if (!flow || !flow.response) return
        flow.waitIntercept = msg.waitIntercept
        flow.response.body = msg.content
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
    const response = flow.response || {}

    return (
      <div className="flow-detail">
        <div className="header-tabs">
          <span onClick={() => { this.setState({ flow: null }) }}>x</span>
          <span className={flowTab === 'Headers' ? 'selected' : null} onClick={() => { this.setState({ flowTab: 'Headers' }) }}>Headers</span>
          <span className={flowTab === 'Preview' ? 'selected' : null} onClick={() => { this.setState({ flowTab: 'Preview' }) }}>Preview</span>
          <span className={flowTab === 'Response' ? 'selected' : null} onClick={() => { this.setState({ flowTab: 'Response' }) }}>Response</span>
          {
            !flow.waitIntercept ? null :
            <div className="flow-wait-area">
              <Button size="sm" onClick={() => {
                const msgType = flow.response ? sendMessageEnum.changeResponse : sendMessageEnum.changeRequest
                const msg = buildMessageEdit(msgType, flow)
                this.ws.send(msg)
                flow.waitIntercept = false
                this.setState({ flows: this.state.flows })
              }}>Continue</Button>
              <Button size="sm" onClick={() => {
                const msgType = flow.response ? sendMessageEnum.dropResponse : sendMessageEnum.dropRequest
                const msg = buildMessageEdit(msgType, flow)
                this.ws.send(msg)
                flow.waitIntercept = false
                this.setState({ flows: this.state.flows })
              }}>Drop</Button>
            </div>
          }
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
                        !(isTextResponse(request)) ? "Not text" :
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
            !(isTextResponse(response)) ? <div>Not text response</div> :
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
            const msg = buildMessageMeta(sendMessageEnum.changeBreakPointRules, rules)
            this.ws.send(msg)
          }} />
        </div>

        <Table striped bordered size="sm">
          <thead>
            <tr>
              <th>No</th>
              <th>Host</th>
              <th>Path</th>
              <th>Method</th>
              <th>Status</th>
              <th>Size</th>
            </tr>
          </thead>
          <tbody>
            {
              flows.map(f => {
                const url = f.request.url
                const u = new URL(url)
                let host = u.host
                if (host.length > 35) host = host.slice(0, 35) + '...'
                let path = u.pathname + u.search
                if (path.length > 65) path = path.slice(0, 65) + '...'

                const request = f.request
                const response = f.response || {}

                const classNames = []
                if (this.state.flow && this.state.flow.id === f.id) classNames.push('tr-selected')
                if (f.waitIntercept) classNames.push('tr-wait-intercept')

                return (
                  <tr className={classNames.length ? classNames.join(' ') : null} key={f.id}
                    onClick={() => {
                      this.setState({ flow: f })
                    }}
                  >
                    <td>{f.no}</td>
                    <td>{host}</td>
                    <td>{path}</td>
                    <td>{request.method}</td>
                    <td>{response.statusCode || '(pending)'}</td>
                    <td>{getSize(response)}</td>
                  </tr>
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

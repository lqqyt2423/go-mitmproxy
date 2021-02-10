import React from 'react'
import Table from 'react-bootstrap/Table'
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import { FlowManager } from './flow'
import './App.css'

const isTextResponse = response => {
  if (!response) return false
  if (!response.header) return false
  if (!response.header['Content-Type']) return false

  return /text|javascript|json/.test(response.header['Content-Type'].join(''))
}

const getSize = response => {
  if (!response) return '0'
  if (!response.header) return '0'
  if (!response.header['Content-Length']) return '0'
  const len = parseInt(response.header['Content-Length'][0])
  if (isNaN(len)) return '0'
  if (len <= 0) return '0'
  
  if (len < 1024) return `${len} B`
  if (len < 1024*1024) return `${(len/1024).toFixed(2)} KB`
  return `${(len/(1024*1024)).toFixed(2)} MB`
}

const parseMessage = data => {
  if (data.byteLength < 38) return null
  const meta = new Int8Array(data.slice(0, 2))
  const version = meta[0]
  if (version !== 1) return null
  const type = meta[1]
  if (![1, 2, 3].includes(type)) return null
  const id = new TextDecoder().decode(data.slice(2, 38))

  const resp = {
    type: ['request', 'response', 'responseBody'][type-1],
    id,
  }
  if (data.byteLength === 38) return resp
  if (type === 3) {
    resp.content = data.slice(38)
    return resp
  }

  let content = new TextDecoder().decode(data.slice(38))
  try {
    content = JSON.parse(content)
  } catch (err) {
    return null
  }

  resp.content = content
  return resp
}

/**
 * 
 * @param {number} messageType 
 * @param {string} id 
 * @param {string} content 
 */
const buildMessage = (messageType, id, content) => {
  content = new TextEncoder().encode(content)
  const data = new Uint8Array(38 + content.byteLength)
  data[0] = 1
  data[1] = messageType
  data.set(new TextEncoder().encode(id), 2)
  data.set(content, 38)
  return data
}

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

    let host;
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
      console.log('msg:', msg)

      if (msg.type === 'request') {
        const flow = { id: msg.id, request: msg.content }
        this.flowMgr.add(flow)
        this.setState({ flows: this.flowMgr.showList() })
      }
      else if (msg.type === 'response') {
        const flow = this.flowMgr.get(msg.id)
        if (!flow) return
        flow.response = msg.content
        this.setState({ flows: this.state.flows })
      }
      else if (msg.type === 'responseBody') {
        const flow = this.flowMgr.get(msg.id)
        if (!flow || !flow.response) return
        flow.response.body = msg.content
        this.setState({ flows: this.state.flows })
      }
    }
    this.ws.onerror = evt => {
      console.log('ERROR:', evt)
    }

    // this.ws.send('msg')
    // this.ws.close()
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

              <div className="header-block">
                <p>Response Headers</p>
                <div className="header-block-content">
                  {
                    !(response.header) ? null :
                    Object.keys(response.header).map(key => {
                      return (
                        <p key={key}>{key}: {response.header[key].join(' ')}</p>
                      )
                    })
                  }
                </div>
              </div>

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
                return (
                  <tr className={(this.state.flow && this.state.flow.id === f.id) ? "tr-selected" : null} key={f.id} onClick={() => {
                    this.setState({ flow: f })
                  }}>
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

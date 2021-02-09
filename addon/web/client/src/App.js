import React from 'react'
import Table from 'react-bootstrap/Table'
import { Base64 } from 'js-base64'
import './App.css'

const isTextResponse = response => {
  if (!response) return false
  if (!response.header) return false
  if (!response.header['Content-Type']) return false

  return /text|javascript|json/.test(response.header['Content-Type'].join(''))
}

class App extends React.Component {

  constructor(props) {
    super(props)

    this.state = {
      flows: [],
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

    this.ws = new WebSocket("ws://localhost:9081/echo")
    this.ws.onopen = () => { console.log('OPEN') }
    this.ws.onclose = () => { console.log('CLOSE') }
    this.ws.onmessage = evt => {
      const data = JSON.parse(evt.data)
      console.log(data)

      const flow = data.flow
      const id = flow.id
      if (data.on === 'request') {
        this.setState({ flows: this.state.flows.concat(flow) })
      }
      else if (data.on === 'response') {
        const flows = this.state.flows.map(f => {
          if (f.id === id) return flow
          return f
        })
        this.setState({ flows })
      }
    }
    this.ws.onerror = evt => {
      console.log('ERROR: ' + evt.data)
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
            !(response.body && response.body.length) ? <div>No response</div> :
            !(isTextResponse(response)) ? <div>Not text response</div> :
            <div>
              {Base64.decode(response.body)}
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
        <Table striped bordered size="sm">
          <thead>
            <tr>
              <th>Host</th>
              <th>Path</th>
              <th>Method</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {
              flows.map(f => {
                const url = f.request.url
                const u = new URL(url)
                let path = u.pathname + u.search
                if (path.length > 60) path = path.slice(0, 60) + '...'

                const request = f.request
                const response = f.response || {}
                return (
                  <tr key={f.id} onClick={() => {
                    this.setState({ flow: f })
                  }}>
                    <td>{u.host}</td>
                    <td>{path}</td>
                    <td>{request.method}</td>
                    <td>{response.statusCode || '(pending)'}</td>
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

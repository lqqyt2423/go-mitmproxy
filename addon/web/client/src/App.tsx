import React from 'react'
import Table from 'react-bootstrap/Table'
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import scrollMonitor from 'scrollmonitor'
import './App.css'

import BreakPoint from './components/BreakPoint'
import FlowPreview from './components/FlowPreview'
import ViewFlow from './components/ViewFlow'

import { FlowManager } from './flow'
import { parseMessage, SendMessageType, buildMessageMeta, Flow, MessageType } from './message'

interface IState {
  flows: Flow[]
  flow: Flow | null
  flowTab: 'Headers' | 'Preview' | 'Response'
  wsStatus: 'open' | 'close' | 'connecting'
}

const wsReconnIntervals = [1, 1, 2, 2, 4, 4, 8, 8, 16, 16, 32, 32]

interface IProps {
  pageBottom: HTMLElement
}

class App extends React.Component<IProps, IState> {
  private flowMgr: FlowManager
  private ws: WebSocket | null
  private wsUnmountClose: boolean

  private autoScore = false

  private wsReconnCount = -1

  constructor(props: IProps) {
    super(props)

    this.flowMgr = new FlowManager()

    this.state = {
      flows: this.flowMgr.showList(),
      flow: null,
      flowTab: 'Headers', // Headers, Preview, Response
      wsStatus: 'close',
    }

    this.ws = null
    this.wsUnmountClose = false

    this.initScrollMonitor()
  }

  componentDidMount() {
    this.initWs()
  }

  componentWillUnmount() {
    if (this.ws) {
      this.wsUnmountClose = true
      this.ws.close()
      this.ws = null
    }
  }

  initWs() {
    if (this.ws) return

    this.setState({ wsStatus: 'connecting' })

    let host
    if (process.env.NODE_ENV === 'development') {
      host = 'localhost:9081'
    } else {
      host = new URL(document.URL).host
    }
    this.ws = new WebSocket(`ws://${host}/echo`)
    this.ws.binaryType = 'arraybuffer'

    this.ws.onopen = () => {
      this.wsReconnCount = -1
      this.setState({ wsStatus: 'open' })
    }

    this.ws.onerror = evt => {
      console.error('ERROR:', evt)
      this.ws?.close()
    }

    this.ws.onclose = () => {
      this.setState({ wsStatus: 'close' })
      if (this.wsUnmountClose) return

      this.wsReconnCount++
      this.ws = null
      const waitSeconds = wsReconnIntervals[this.wsReconnCount] || wsReconnIntervals[wsReconnIntervals.length - 1]
      console.info(`will reconnect after ${waitSeconds} seconds`)
      setTimeout(() => {
        this.initWs()
      }, waitSeconds * 1000)
    }

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
        this.setState({ flows: this.flowMgr.showList() }, () => {
          if (this.autoScore) {
            this.props.pageBottom.scrollIntoView({ behavior: 'auto' })
          }
        })
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
  }

  initScrollMonitor() {
    const watcher = scrollMonitor.create(this.props.pageBottom)
    watcher.enterViewport(() => {
      this.autoScore = true
    })
    watcher.exitViewport(() => {
      this.autoScore = false
    })
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

          <span>status: {this.state.wsStatus}</span>
        </div>

        <Table striped bordered size="sm" style={{ tableLayout: 'fixed' }}>
          <thead>
            <tr>
              <th style={{ width: '50px' }}>No</th>
              <th style={{ width: '80px' }}>Method</th>
              <th style={{ width: '200px' }}>Host</th>
              <th style={{ width: 'auto' }}>Path</th>
              <th style={{ width: '150px' }}>Type</th>
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

        <ViewFlow
          flow={this.state.flow}
          onClose={() => { this.setState({ flow: null }) }}
          onReRenderFlows={() => { this.setState({ flows: this.state.flows }) }}
          onMessage={msg => { if (this.ws) this.ws.send(msg) }}
        />
      </div>
    )
  }
}

export default App

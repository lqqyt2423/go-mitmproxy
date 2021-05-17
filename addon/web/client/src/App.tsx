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
}

class App extends React.Component<any, IState> {
  private flowMgr: FlowManager
  private ws: WebSocket | null

  private pageBottom: HTMLDivElement | null
  private autoScore = false

  constructor(props: any) {
    super(props)

    this.flowMgr = new FlowManager()

    this.state = {
      flows: this.flowMgr.showList(),
      flow: null,

      flowTab: 'Headers', // Headers, Preview, Response
    }

    this.ws = null
    this.pageBottom = null
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
        this.setState({ flows: this.flowMgr.showList() }, () => {
          if (this.pageBottom && this.autoScore) this.pageBottom.scrollIntoView({ behavior: 'auto' })
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
    this.ws.onerror = evt => {
      console.log('ERROR:', evt)
    }
  }

  initScrollMonitor() {
    if (!this.pageBottom) return

    const watcher = scrollMonitor.create(this.pageBottom)
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
        </div>

        <Table striped bordered size="sm" style={{ tableLayout: 'fixed' }}>
          <thead>
            <tr>
              <th style={{ width: '50px' }}>No</th>
              <th style={{ width: '80px' }}>Method</th>
              <th style={{ width: '200px' }}>Host</th>
              <th style={{ width: '600px' }}>Path</th>
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

        <div ref={el => {
          if (this.pageBottom) return
          this.pageBottom = el
          this.initScrollMonitor()
        }} style={{ height: '0px', visibility: 'hidden' }}>bottom</div>
      </div>
    )
  }
}

export default App

import React from 'react'
import Table from 'react-bootstrap/Table'
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import './App.css'
import GitHubLogo from './github-mark.svg'
import Badge from 'react-bootstrap/Badge'

import BreakPoint from './containers/BreakPoint'
import FlowPreview from './containers/FlowPreview'
import ViewFlow from './containers/ViewFlow'
import Resizer from './components/Resizer'

import { Flow, FlowManager } from './utils/flow'
import { parseMessage, SendMessageType, buildMessageMeta, MessageType } from './utils/message'
import { isInViewPort } from './utils/utils'
import { ConnectionManager, IConnection } from './utils/connection'

interface IState {
  flows: Flow[]
  flow: Flow | null
  wsStatus: 'open' | 'close' | 'connecting'
  filterInvalid: boolean
}

const wsReconnIntervals = [1, 1, 2, 2, 4, 4, 8, 8, 16, 16, 32, 32]

// eslint-disable-next-line @typescript-eslint/no-empty-interface
interface IProps { }

class App extends React.Component<IProps, IState> {
  private connMgr: ConnectionManager
  private flowMgr: FlowManager
  private ws: WebSocket | null
  private pendingMessages: Array<string | ArrayBufferLike | Blob | ArrayBufferView>
  private wsUnmountClose: boolean
  private tableBottomRef: React.RefObject<HTMLDivElement>

  private wsReconnCount = -1

  constructor(props: IProps) {
    super(props)

    this.connMgr = new ConnectionManager()
    this.flowMgr = new FlowManager()

    this.state = {
      flows: this.flowMgr.showList(),
      flow: null,
      wsStatus: 'close',
      filterInvalid: false,
    }

    this.ws = null
    this.pendingMessages = []
    this.wsUnmountClose = false
    this.tableBottomRef = React.createRef<HTMLDivElement>()
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

      for (const msg of this.pendingMessages) {
        this.ws?.send(msg)
      }
      this.pendingMessages = []
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

      if (msg.type === MessageType.CONN) {
        const conn = msg.content as IConnection
        if (conn.intercept) conn.opening = true
        this.connMgr.add(msg.id, conn)
        this.setState({ flows: this.state.flows })
      }
      else if (msg.type === MessageType.CONN_CLOSE) {
        const conn = this.connMgr.get(msg.id)
        if (!conn) return
        conn.opening = false
        conn.flowCount = msg.content as number
        this.setState({ flows: this.state.flows })
        this.connMgr.delete(msg.id)
      }
      else if (msg.type === MessageType.REQUEST) {
        let flow = this.flowMgr.get(msg.id)
        if (!flow) {
          flow = new Flow(msg, this.connMgr)
          flow.getConn()
          this.flowMgr.add(flow)
  
          let shouldScroll = false
          if (this.tableBottomRef?.current && isInViewPort(this.tableBottomRef.current)) {
            shouldScroll = true
          }
          this.setState({ flows: this.flowMgr.showList() }, () => {
            if (shouldScroll) {
              this.tableBottomRef?.current?.scrollIntoView({ behavior: 'auto' })
            }
          })
        } else {
          flow.addRequest(msg)
          flow.getConn()
          this.setState({ flows: this.state.flows })
        }
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
        flow.getConn()
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

  wsSend(msg: string | ArrayBufferLike | Blob | ArrayBufferView) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(msg)
      return
    }
    this.pendingMessages.push(msg)
    // 先最多保留10条
    if (this.pendingMessages.length > 10) {
      this.pendingMessages = this.pendingMessages.slice(this.pendingMessages.length - 10)
    }
  }

  render() {
    const { flows } = this.state
    return (
      <div className="main-table-wrap">
        <div className="top-control">
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <div style={{ marginRight: '10px' }}><Button size="sm" onClick={() => {
              this.flowMgr.clear()
              this.setState({ flows: this.flowMgr.showList(), flow: null })
            }}>Clear</Button></div>
            <div style={{ marginRight: '10px' }}>
              <Form.Control
                size="sm" placeholder="Filter"
                style={{ width: '350px' }}
                isInvalid={this.state.filterInvalid}
                onChange={(e) => {
                  const value = e.target.value
                  this.flowMgr.changeFilterLazy(value, (err) => {
                    if (err) {
                      console.log('changeFilterLazy error', err)
                    }
                    this.setState({
                      filterInvalid: err ? true : false,
                      flows: this.flowMgr.showList()
                    })
                  })
                }}
              >
              </Form.Control>
            </div>
            
            <div style={{ marginRight: '10px' }}>
              <BreakPoint onSave={rules => {
                const msg = buildMessageMeta(SendMessageType.CHANGE_BREAK_POINT_RULES, rules)
                this.wsSend(msg)
              }} />
            </div>
          </div>
          
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <div style={{ marginRight: '10px' }}>
              { this.state.wsStatus === 'open' ? <Badge pill bg="success">on</Badge> : <Badge pill bg="danger">off</Badge> }
            </div>
            <a href='https://github.com/lqqyt2423/go-mitmproxy' target='_blank' rel="noreferrer"><img style={{ height: '30px' }} src={GitHubLogo} alt="GitHub Logo" /></a>
          </div>
        </div>

        <div className="table-wrap-div">
          <Table striped size="sm" style={{ tableLayout: 'fixed' }}>
            <thead>
              <tr>
                <Resizer width={50}>No</Resizer>
                <Resizer width={80}>Method</Resizer>
                <Resizer width={250}>Host</Resizer>
                <Resizer width={500}>Path</Resizer>
                <Resizer width={150}>Type</Resizer>
                <Resizer width={80}>Status</Resizer>
                <Resizer width={90}>Size</Resizer>
                <Resizer width={90}>Time</Resizer>
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
          <div ref={this.tableBottomRef} id="hidden-bottom" style={{ height: '0px', visibility: 'hidden', marginBottom: '1px' }}></div>
        </div>

        <ViewFlow
          flow={this.state.flow}
          onClose={() => { this.setState({ flow: null }) }}
          onReRenderFlows={() => { this.setState({ flows: this.state.flows }) }}
          onMessage={msg => { this.wsSend(msg) }}
        />
      </div>
    )
  }
}

export default App

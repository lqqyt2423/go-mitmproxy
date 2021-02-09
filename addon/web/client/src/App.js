import React from 'react'
import Table from 'react-bootstrap/Table'
import './App.css'

class App extends React.Component {

  constructor(props) {
    super(props)

    this.state = {
      flows: [],
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
  
  render() {
    const { flows } = this.state
    return (
      <div className="main-table-wrap">
        <Table striped bordered size="sm">
          <thead>
            <tr>
              <th>Status</th>
              <th>Method</th>
              <th>Host</th>
              <th>Path</th>
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
                  <tr key={f.id}>
                    <td>{response.statusCode || '(pending)'}</td>
                    <td>{request.method}</td>
                    <td>{u.host}</td>
                    <td>{path}</td>
                  </tr>
                )
              })
            }
          </tbody>
        </Table>
      </div>
    )
  }
}

export default App

import React from 'react'
import Button from 'react-bootstrap/Button'
import Modal from 'react-bootstrap/Modal'
import Form from 'react-bootstrap/Form'
import Alert from 'react-bootstrap/Alert'

import { sendMessageEnum, buildMessageEdit } from '../message'
import { isTextBody } from '../utils'


const stringifyRequest = request => {
  const firstLine = `${request.method} ${request.url}`
  const headerLines = Object.keys(request.header).map(key => {
    const valstr = request.header[key].join(' \t ') // for parse convenience
    return `${key}: ${valstr}`
  }).join('\n')

  let bodyLines = ''
  if (request.body && isTextBody(request)) bodyLines = new TextDecoder().decode(request.body)
  
  return `${firstLine}\n\n${headerLines}\n\n${bodyLines}`
}

const parseRequest = content => {
  const sections = content.split('\n\n')
  if (sections.length !== 3) return

  const [firstLine, headerLines, bodyLines] = sections
  const [method, url] = firstLine.split(' ')
  if (!method || !url) return
  
  const header = {}
  for (const line of headerLines.split('\n')) {
    const [key, vals] = line.split(': ')
    if (!key || !vals) return
    header[key] = vals.split(' \t ')
  }

  let body = null
  if (bodyLines) body = new TextEncoder().encode(bodyLines)

  return {
    method,
    url,
    header,
    body,
  }
}

const stringifyResponse = response => {
  const firstLine = `${response.statusCode}`
  const headerLines = Object.keys(response.header).map(key => {
    const valstr = response.header[key].join(' \t ') // for parse convenience
    return `${key}: ${valstr}`
  }).join('\n')

  let bodyLines = ''
  if (response.body && isTextBody(response)) bodyLines = new TextDecoder().decode(response.body)
  
  return `${firstLine}\n\n${headerLines}\n\n${bodyLines}`
}

const parseResponse = content => {
  const sections = content.split('\n\n')
  if (sections.length !== 3) return

  const [firstLine, headerLines, bodyLines] = sections
  const statusCode = parseInt(firstLine)
  if (isNaN(statusCode)) return

  const header = {}
  for (const line of headerLines.split('\n')) {
    const [key, vals] = line.split(': ')
    if (!key || !vals) return
    header[key] = vals.split(' \t ')
  }

  let body = null
  if (bodyLines) body = new TextEncoder().encode(bodyLines)

  return {
    statusCode,
    header,
    body,
  }
}


class EditFlow extends React.Component {
  constructor(props) {
    super(props)

    this.state = {
      show: false,
      alertMsg: '',
      content: '',
    }

    this.handleClose = this.handleClose.bind(this)
    this.handleShow = this.handleShow.bind(this)
    this.handleSave = this.handleSave.bind(this)
  }

  showAlert(msg) {
    this.setState({ alertMsg: msg })
  }

  handleClose() {
    this.setState({ show: false })
  }

  handleShow() {
    const { flow } = this.props
    const when = flow.response ? 'response' : 'request'

    let content = ''
    if (when === 'request') {
      content = stringifyRequest(flow.request)
    } else {
      content = stringifyResponse(flow.response)
    }

    this.setState({ show: true, alertMsg: '', content })
  }

  handleSave() {
    const { flow } = this.props
    const when = flow.response ? 'response' : 'request'

    const { content } = this.state

    if (when === 'request') {
      const request = parseRequest(content)
      if (!request) {
        this.showAlert('parse error')
        return
      }

      this.props.onChangeRequest(request)
      this.handleClose()
    } else {
      const response = parseResponse(content)
      if (!response) {
        this.showAlert('parse error')
        return
      }

      this.props.onChangeResponse(response)
      this.handleClose()
    }
  }

  render() {
    const { flow } = this.props
    if (!flow.waitIntercept) return null

    const { alertMsg } = this.state
    
    const when = flow.response ? 'response' : 'request'

    return (
      <div className="flow-wait-area">

        <Button size="sm" onClick={this.handleShow}>Edit</Button>

        <Button size="sm" onClick={() => {
          const msgType = when === 'response' ? sendMessageEnum.changeResponse : sendMessageEnum.changeRequest
          const msg = buildMessageEdit(msgType, flow)
          this.props.onMessage(msg)
        }}>Continue</Button>

        <Button size="sm" onClick={() => {
          const msgType = when === 'response' ? sendMessageEnum.dropResponse : sendMessageEnum.dropRequest
          const msg = buildMessageEdit(msgType, flow)
          this.props.onMessage(msg)
        }}>Drop</Button>


        <Modal size="lg" show={this.state.show} onHide={this.handleClose}>
          <Modal.Header closeButton>
            <Modal.Title>Edit {when === 'request' ? 'Request' : 'Response'}</Modal.Title>
          </Modal.Header>

          <Modal.Body>
            <Form.Group>
              <Form.Control as="textarea" rows={10} value={this.state.content} onChange={e => { this.setState({ content: e.target.value }) }} />
            </Form.Group>
            {
              !alertMsg ? null : <Alert variant="danger">{alertMsg}</Alert>
            }
          </Modal.Body>

          <Modal.Footer>
            <Button variant="secondary" onClick={this.handleClose}>
              Close
            </Button>
            <Button variant="primary" onClick={this.handleSave}>
              Save
            </Button>
          </Modal.Footer>
        </Modal>

      </div>
    )
  }
}

export default EditFlow

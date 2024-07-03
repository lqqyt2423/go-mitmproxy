import React, { useState } from 'react'
import Button from 'react-bootstrap/Button'
import Modal from 'react-bootstrap/Modal'
import Form from 'react-bootstrap/Form'
import Alert from 'react-bootstrap/Alert'
import { SendMessageType, buildMessageEdit } from '../utils/message'
import { isTextBody } from '../utils/utils'
import type { Flow, Header, IRequest, IResponse } from '../utils/flow'

const stringifyRequest = (request: IRequest) => {
  const firstLine = `${request.method} ${request.url}`
  const headerLines = Object.keys(request.header).map(key => {
    const valstr = request.header[key].join(' \t ') // for parse convenience
    return `${key}: ${valstr}`
  }).join('\n')

  let bodyLines = ''
  if (request.body && isTextBody(request)) bodyLines = new TextDecoder().decode(request.body)

  return `${firstLine}\n\n${headerLines}\n\n${bodyLines}`
}

const parseRequest = (content: string): IRequest | undefined => {
  const firstIndex = content.indexOf('\n\n')
  if (firstIndex <= 0) return

  const firstLine = content.slice(0, firstIndex)
  const [method, url] = firstLine.split(' ')
  if (!method || !url) return

  const secondIndex = content.indexOf('\n\n', firstIndex + 2)
  if (secondIndex <= 0) return
  const headerLines = content.slice(firstIndex + 2, secondIndex)
  const header: Header = {}
  for (const line of headerLines.split('\n')) {
    const [key, vals] = line.split(': ')
    if (!key || !vals) return
    header[key] = vals.split(' \t ')
  }

  const bodyLines = content.slice(secondIndex + 2)
  let body: ArrayBuffer | undefined
  if (bodyLines) body = new TextEncoder().encode(bodyLines)

  return {
    method,
    url,
    proto: '',
    header,
    body,
  }
}

const stringifyResponse = (response: IResponse) => {
  const firstLine = `${response.statusCode}`
  const headerLines = Object.keys(response.header).map(key => {
    const valstr = response.header[key].join(' \t ') // for parse convenience
    return `${key}: ${valstr}`
  }).join('\n')

  let bodyLines = ''
  if (response.body && isTextBody(response)) bodyLines = new TextDecoder().decode(response.body)

  return `${firstLine}\n\n${headerLines}\n\n${bodyLines}`
}

const parseResponse = (content: string): IResponse | undefined => {
  const firstIndex = content.indexOf('\n\n')
  if (firstIndex <= 0) return

  const firstLine = content.slice(0, firstIndex)
  const statusCode = parseInt(firstLine)
  if (isNaN(statusCode)) return

  const secondIndex = content.indexOf('\n\n', firstIndex + 2)
  if (secondIndex <= 0) return
  const headerLines = content.slice(firstIndex + 2, secondIndex)
  const header: Header = {}
  for (const line of headerLines.split('\n')) {
    const [key, vals] = line.split(': ')
    if (!key || !vals) return
    header[key] = vals.split(' \t ')
  }

  const bodyLines = content.slice(secondIndex + 2)
  let body: ArrayBuffer | undefined
  if (bodyLines) body = new TextEncoder().encode(bodyLines)

  return {
    statusCode,
    header,
    body,
  }
}


interface IProps {
  flow: Flow
  onChangeRequest: (request: IRequest) => void
  onChangeResponse: (response: IResponse) => void
  onMessage: (msg: ArrayBufferLike) => void
}

function EditFlow({ flow, onChangeRequest, onChangeResponse, onMessage }: IProps) {
  const when = flow.response ? 'response' : 'request'

  const [show, setShow] = useState(false)
  const [alertMsg, setAlertMsg] = useState('')
  const [content, setContent] = useState('')

  const showAlert = (msg: string) => setAlertMsg(msg)
  const handleClose = () => setShow(false)
  
  const handleShow = () => {
    let content = ''
    if (when === 'request') {
      content = stringifyRequest(flow.request)
    } else {
      content = stringifyResponse(flow.response as IResponse)
    }

    setAlertMsg('')
    setContent(content)
    setShow(true)
  }

  const handleSave = () => {
    if (when === 'request') {
      const request = parseRequest(content)
      if (!request) {
        showAlert('parse error')
        return
      }
      onChangeRequest(request)
      handleClose()
    } else {
      const response = parseResponse(content)
      if (!response) {
        showAlert('parse error')
        return
      }
      onChangeResponse(response)
      handleClose()
    }
  }

  if (!flow.waitIntercept) return null
  return (
    <div className="flow-wait-area">

      <Button size="sm" onClick={handleShow}>Edit</Button>

      <Button size="sm" onClick={() => {
        const msgType = when === 'response' ? SendMessageType.CHANGE_RESPONSE : SendMessageType.CHANGE_REQUEST
        const msg = buildMessageEdit(msgType, flow)
        onMessage(msg)
      }}>Continue</Button>

      <Button size="sm" onClick={() => {
        const msgType = when === 'response' ? SendMessageType.DROP_RESPONSE : SendMessageType.DROP_REQUEST
        const msg = buildMessageEdit(msgType, flow)
        onMessage(msg)
      }}>Drop</Button>


      <Modal size="lg" show={show} onHide={handleClose}>
        <Modal.Header closeButton>
          <Modal.Title>Edit {when === 'request' ? 'Request' : 'Response'}</Modal.Title>
        </Modal.Header>

        <Modal.Body>
          <Form.Group>
            <Form.Control as="textarea" rows={10} value={content} onChange={e => { setContent(e.target.value) }} />
          </Form.Group>
          {
            !alertMsg ? null : <Alert variant="danger">{alertMsg}</Alert>
          }
        </Modal.Body>

        <Modal.Footer>
          <Button variant="secondary" onClick={handleClose}>
            Close
          </Button>
          <Button variant="primary" onClick={handleSave}>
            Save
          </Button>
        </Modal.Footer>
      </Modal>

    </div>
  )
}

export default EditFlow

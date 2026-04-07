import React, { useState } from 'react'
import Button from 'react-bootstrap/Button'
import Form from 'react-bootstrap/Form'
import Modal from 'react-bootstrap/Modal'
import { SendMessageType } from '../utils/message'

const MESSAGE_VERSION = 2

interface IProps {
  show: boolean
  onClose: () => void
  onSend: (msg: ArrayBufferLike) => void
}

function Compose({ show, onClose, onSend }: IProps) {
  const [method, setMethod] = useState('GET')
  const [url, setUrl] = useState('')
  const [headers, setHeaders] = useState('')
  const [body, setBody] = useState('')

  const handleSend = () => {
    if (!url) return

    const headerObj: Record<string, string[]> = {}
    headers.split('\n').forEach(line => {
      const idx = line.indexOf(':')
      if (idx > 0) {
        const key = line.substring(0, idx).trim()
        const val = line.substring(idx + 1).trim()
        if (!headerObj[key]) headerObj[key] = []
        headerObj[key].push(val)
      }
    })

    const json = JSON.stringify({ method, url, header: headerObj })
    const jsonBytes = new TextEncoder().encode(json)
    const bodyBytes = new TextEncoder().encode(body)

    // version(1) + type(1) + id(36) + headerLen(4) + header + bodyLen(4) + body
    const idBytes = new TextEncoder().encode('00000000-0000-0000-0000-000000000000')
    const buf = new ArrayBuffer(2 + 36 + 4 + jsonBytes.length + 4 + bodyBytes.length)
    const view = new DataView(buf)
    const u8 = new Uint8Array(buf)

    u8[0] = MESSAGE_VERSION
    u8[1] = SendMessageType.COMPOSE_REQUEST
    u8.set(idBytes, 2)
    view.setUint32(38, jsonBytes.length)
    u8.set(jsonBytes, 42)
    view.setUint32(42 + jsonBytes.length, bodyBytes.length)
    u8.set(bodyBytes, 46 + jsonBytes.length)

    onSend(buf)
    onClose()
  }

  return (
    <Modal show={show} onHide={onClose} size="lg">
      <Modal.Header closeButton>
        <Modal.Title>Compose Request</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <Form.Group className="mb-3">
          <Form.Label>Method</Form.Label>
          <Form.Select size="sm" value={method} onChange={e => setMethod(e.target.value)}>
            <option>GET</option>
            <option>POST</option>
            <option>PUT</option>
            <option>DELETE</option>
            <option>PATCH</option>
            <option>HEAD</option>
            <option>OPTIONS</option>
          </Form.Select>
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>URL</Form.Label>
          <Form.Control size="sm" placeholder="https://example.com/api" value={url} onChange={e => setUrl(e.target.value)} />
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>Headers (one per line, Key: Value)</Form.Label>
          <Form.Control as="textarea" rows={3} size="sm" placeholder="Content-Type: application/json" value={headers} onChange={e => setHeaders(e.target.value)} />
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>Body</Form.Label>
          <Form.Control as="textarea" rows={5} size="sm" value={body} onChange={e => setBody(e.target.value)} />
        </Form.Group>
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" size="sm" onClick={onClose}>Cancel</Button>
        <Button variant="primary" size="sm" onClick={handleSend} disabled={!url}>Send</Button>
      </Modal.Footer>
    </Modal>
  )
}

export default Compose

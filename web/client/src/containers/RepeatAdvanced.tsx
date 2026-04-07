import React, { useState } from 'react'
import Button from 'react-bootstrap/Button'
import Form from 'react-bootstrap/Form'
import Modal from 'react-bootstrap/Modal'
import { SendMessageType } from '../utils/message'

const MESSAGE_VERSION = 2

interface IProps {
  show: boolean
  flowId: string
  onClose: () => void
  onSend: (msg: ArrayBufferLike) => void
}

function RepeatAdvanced({ show, flowId, onClose, onSend }: IProps) {
  const [count, setCount] = useState(10)
  const [concurrency, setConcurrency] = useState(3)

  const handleStart = () => {
    const config = JSON.stringify({ count, concurrency })
    const configBytes = new TextEncoder().encode(config)
    const idBytes = new TextEncoder().encode(flowId)

    const buf = new ArrayBuffer(2 + 36 + configBytes.length)
    const u8 = new Uint8Array(buf)
    u8[0] = MESSAGE_VERSION
    u8[1] = SendMessageType.REPEAT_ADVANCED
    u8.set(idBytes, 2)
    u8.set(configBytes, 38)

    onSend(buf)
    onClose()
  }

  return (
    <Modal show={show} onHide={onClose}>
      <Modal.Header closeButton>
        <Modal.Title>Repeat Advanced</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <Form.Group className="mb-3">
          <Form.Label>Request Count</Form.Label>
          <Form.Control type="number" size="sm" min={1} max={10000} value={count} onChange={e => setCount(parseInt(e.target.value) || 1)} />
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>Concurrency</Form.Label>
          <Form.Control type="number" size="sm" min={1} max={100} value={concurrency} onChange={e => setConcurrency(parseInt(e.target.value) || 1)} />
        </Form.Group>
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" size="sm" onClick={onClose}>Cancel</Button>
        <Button variant="primary" size="sm" onClick={handleStart}>Start</Button>
      </Modal.Footer>
    </Modal>
  )
}

export default RepeatAdvanced

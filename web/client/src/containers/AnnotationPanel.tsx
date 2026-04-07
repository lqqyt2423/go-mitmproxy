import React, { useState, useEffect } from 'react'
import Button from 'react-bootstrap/Button'
import Form from 'react-bootstrap/Form'
import type { Flow, IFlowAnnotation } from '../utils/flow'
import { SendMessageType } from '../utils/message'

const MESSAGE_VERSION = 2

const COLORS = [
  { name: '', label: 'None', bg: 'transparent' },
  { name: 'red', label: 'Red', bg: '#f8d7da' },
  { name: 'blue', label: 'Blue', bg: '#cfe2ff' },
  { name: 'green', label: 'Green', bg: '#d1e7dd' },
  { name: 'yellow', label: 'Yellow', bg: '#fff3cd' },
  { name: 'purple', label: 'Purple', bg: '#e2d9f3' },
]

interface IProps {
  flow: Flow
  onSend: (msg: ArrayBufferLike) => void
  onUpdate: () => void
}

function AnnotationPanel({ flow, onSend, onUpdate }: IProps) {
  const [color, setColor] = useState(flow.annotation?.color || '')
  const [comment, setComment] = useState(flow.annotation?.comment || '')

  useEffect(() => {
    setColor(flow.annotation?.color || '')
    setComment(flow.annotation?.comment || '')
  }, [flow])

  const sendAnnotation = (newColor: string, newComment: string) => {
    const annotation: IFlowAnnotation = { color: newColor, comment: newComment }
    flow.annotation = annotation

    const json = JSON.stringify(annotation)
    const jsonBytes = new TextEncoder().encode(json)
    const idBytes = new TextEncoder().encode(flow.id)

    const buf = new ArrayBuffer(2 + 36 + jsonBytes.length)
    const u8 = new Uint8Array(buf)
    u8[0] = MESSAGE_VERSION
    u8[1] = SendMessageType.SET_ANNOTATION
    u8.set(idBytes, 2)
    u8.set(jsonBytes, 38)

    onSend(buf)
    onUpdate()
  }

  return (
    <div className="header-block">
      <p>Annotation</p>
      <div className="header-block-content">
        <div style={{ display: 'flex', gap: '4px', marginBottom: '8px' }}>
          {COLORS.map(c => (
            <Button
              key={c.name}
              size="sm"
              variant={color === c.name ? 'dark' : 'outline-secondary'}
              style={{ backgroundColor: color === c.name ? undefined : c.bg, minWidth: '60px' }}
              onClick={() => {
                setColor(c.name)
                sendAnnotation(c.name, comment)
              }}
            >
              {c.label}
            </Button>
          ))}
        </div>
        <Form.Group>
          <Form.Control
            size="sm"
            placeholder="Add comment..."
            value={comment}
            onChange={e => setComment(e.target.value)}
            onBlur={() => sendAnnotation(color, comment)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                sendAnnotation(color, comment)
              }
            }}
          />
        </Form.Group>
      </div>
    </div>
  )
}

export default AnnotationPanel

import React, { useEffect, useState } from 'react'
import Button from 'react-bootstrap/Button'
import Modal from 'react-bootstrap/Modal'
import Form from 'react-bootstrap/Form'
import Row from 'react-bootstrap/Row'
import Col from 'react-bootstrap/Col'
import { BreakPointRuleAction, BreakPointRuleMethod, IBreakPointRule, configBreakPointRule, useConfig } from '../utils/config'

interface IProps {
  onSave: (rules: IBreakPointRule[]) => void
}

function BreakPoint({ onSave }: IProps) {
  const [show, setShow] = useState(false)
  const [rule, setRule] = useConfig(configBreakPointRule)
  const [haveRules, setHaveRules] = useState(false)

  const variant = haveRules ? 'success' : 'primary'

  const handleClose = () => setShow(false)
  const handleShow = () => setShow(true)
  const handleSave = () => {
    const rules: IBreakPointRule[] = []
    if (rule.url) {
      rules.push({
        method: rule.method === 'ALL' ? '' : rule.method,
        url: rule.url,
        action: rule.action,
      })
    }
    onSave(rules)
    handleClose()
    setHaveRules(rules.length ? true : false)
  }

  useEffect(() => {
    handleSave()
  }, [])

  return (
    <div>
      <Button variant={variant} size="sm" onClick={handleShow}>BreakPoint</Button>

      <Modal show={show} onHide={handleClose}>
        <Modal.Header closeButton>
          <Modal.Title>Set BreakPoint</Modal.Title>
        </Modal.Header>

        <Modal.Body>
          <Form.Group as={Row}>
            <Form.Label column sm={2}>Method</Form.Label>
            <Col sm={10}>
              <Form.Control as="select" value={rule.method} onChange={e => { setRule({ ...rule, method: e.target.value as BreakPointRuleMethod }) }}>
                <option>ALL</option>
                <option>GET</option>
                <option>POST</option>
                <option>PUT</option>
                <option>DELETE</option>
              </Form.Control>
            </Col>
          </Form.Group>

          <Form.Group as={Row}>
            <Form.Label column sm={2}>URL</Form.Label>
            <Col sm={10}><Form.Control value={rule.url} onChange={e => { setRule({ ...rule, url: e.target.value }) }} /></Col>
          </Form.Group>

          <Form.Group as={Row}>
            <Form.Label column sm={2}>Action</Form.Label>
            <Col sm={10}>
              <Form.Control as="select" value={rule.action} onChange={e => { setRule({ ...rule, action: parseInt(e.target.value) as BreakPointRuleAction }) }}>
                <option value="1">Request</option>
                <option value="2">Response</option>
                <option value="3">Both</option>
              </Form.Control>
            </Col>
          </Form.Group>
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

export default BreakPoint

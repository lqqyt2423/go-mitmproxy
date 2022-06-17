import React from 'react'
import Button from 'react-bootstrap/Button'
import Modal from 'react-bootstrap/Modal'
import Form from 'react-bootstrap/Form'
import Row from 'react-bootstrap/Row'
import Col from 'react-bootstrap/Col'

type Method = 'ALL' | 'GET' | 'POST' | 'PUT' | 'DELETE' | ''
type Action = 1 | 2 | 3
interface IRule {
  method: Method
  url: string
  action: Action
}

interface IState {
  show: boolean
  rule: IRule
  haveRules: boolean
}

interface IProps {
  onSave: (rules: IRule[]) => void
}

class BreakPoint extends React.Component<IProps, IState> {
  constructor(props: IProps) {
    super(props)

    this.state = {
      show: false,

      rule: {
        method: 'ALL',
        url: '',
        action: 1,
      },

      haveRules: false,
    }

    this.handleClose = this.handleClose.bind(this)
    this.handleShow = this.handleShow.bind(this)
    this.handleSave = this.handleSave.bind(this)
  }

  handleClose() {
    this.setState({ show: false })
  }

  handleShow() {
    this.setState({ show: true })
  }

  handleSave() {
    const { rule } = this.state
    const rules: IRule[] = []
    if (rule.url) {
      rules.push({
        method: rule.method === 'ALL' ? '' : rule.method,
        url: rule.url,
        action: rule.action,
      })
    }

    this.props.onSave(rules)
    this.handleClose()

    this.setState({ haveRules: rules.length ? true : false })
  }

  render() {
    const { rule, haveRules } = this.state
    const variant = haveRules ? 'success' : 'primary'

    return (
      <div>
        <Button variant={variant} size="sm" onClick={this.handleShow}>BreakPoint</Button>

        <Modal show={this.state.show} onHide={this.handleClose}>
          <Modal.Header closeButton>
            <Modal.Title>Set BreakPoint</Modal.Title>
          </Modal.Header>

          <Modal.Body>
            <Form.Group as={Row}>
              <Form.Label column sm={2}>Method</Form.Label>
              <Col sm={10}>
                <Form.Control as="select" value={rule.method} onChange={e => { this.setState({ rule: { ...rule, method: e.target.value as Method } }) }}>
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
              <Col sm={10}><Form.Control value={rule.url} onChange={e => { this.setState({ rule: { ...rule, url: e.target.value } }) }} /></Col>
            </Form.Group>

            <Form.Group as={Row}>
              <Form.Label column sm={2}>Action</Form.Label>
              <Col sm={10}>
                <Form.Control as="select" value={rule.action} onChange={e => { this.setState({ rule: { ...rule, action: parseInt(e.target.value) as Action } }) }}>
                  <option value="1">Request</option>
                  <option value="2">Response</option>
                  <option value="3">Both</option>
                </Form.Control>
              </Col>
            </Form.Group>
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

export default BreakPoint

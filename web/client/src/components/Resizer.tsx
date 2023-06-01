import React from 'react'

// https://htmldom.dev/resize-columns-of-a-table/

interface IProps {
  children: React.ReactNode
  width: number
}

interface IState {
  width: number
}

class Resizer extends React.Component<IProps, IState> {
  x = 0
  w = 0

  constructor(props: IProps) {
    super(props)
    this.state = {
      width: props.width
    }

    this.mouseMoveHandler = this.mouseMoveHandler.bind(this)
    this.mouseUpHandler = this.mouseUpHandler.bind(this)
  }

  mouseMoveHandler(e: any) {
    const dx = e.clientX - this.x
    this.setState({ width: this.w + dx })
  }

  mouseUpHandler() {
    document.removeEventListener('mousemove', this.mouseMoveHandler)
    document.removeEventListener('mouseup', this.mouseUpHandler)
  }

  render() {
    return (
      <th
        style={{
          width: `${this.state.width}px`,
          position: 'relative',
        }}
      >
        {this.props.children}
        <div
          style={{
            position: 'absolute',
            top: 0,
            right: 0,
            width: '5px',
            height: '100%',
            cursor: 'col-resize',
            userSelect: 'none',
          }}
          onMouseDown={(e) => {
            this.x = e.clientX
            this.w = this.state.width

            document.addEventListener('mousemove', this.mouseMoveHandler)
            document.addEventListener('mouseup', this.mouseUpHandler)
          }}
        ></div>
      </th>
    )
  }
}

export default Resizer

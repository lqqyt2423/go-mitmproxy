import React from 'react'

// https://htmldom.dev/resize-columns-of-a-table/

interface Iprops {
  children: React.ReactNode;
  width: string;
}

// eslint-disable-next-line @typescript-eslint/no-empty-interface
interface IState {}

class Resizer extends React.Component<Iprops, IState> {
  x = 0
  w = 0

  render() {
    return (
      <th
        style={{
          width: this.props.width,
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
            // this.x = e.clientX
          }}
        ></div>
      </th>
    )
  }
}

export default Resizer

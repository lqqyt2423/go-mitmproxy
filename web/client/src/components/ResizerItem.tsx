import React from 'react'

export function ResizerItem({
  width,
  setWidth,
  left = 0,
  minWidth = 10,
  maxWidth = 1000,
}: {
    width: number,
    setWidth: (w: number) => void,
    left: number,
    minWidth: number,
    maxWidth: number,
}) {
  let x = 0
  let w = 0

  function handleMouseDown(e: React.MouseEvent<HTMLDivElement, MouseEvent>) {
    x = e.clientX
    w = width

    document.addEventListener('mousemove', mouseMoveHandler)
    document.addEventListener('mouseup', mouseUpHandler)
  }

  function mouseMoveHandler(e: MouseEvent) {
    const dx = x - e.clientX
    let nextWidth = w + dx
    if (nextWidth < minWidth) nextWidth = minWidth
    if (nextWidth > maxWidth) nextWidth = maxWidth
    setWidth(nextWidth)
  }

  function mouseUpHandler() {
    document.removeEventListener('mousemove', mouseMoveHandler)
    document.removeEventListener('mouseup', mouseUpHandler)
  }

  return (
    <div
      style={{
        position: 'absolute',
        top: '0',
        left: left,
        width: '5px',
        height: '100%',
        cursor: 'col-resize',
        userSelect: 'none',
      }}
      onMouseDown={handleMouseDown}
    ></div>
  )
}

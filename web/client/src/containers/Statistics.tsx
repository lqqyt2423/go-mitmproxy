import React from 'react'
import Table from 'react-bootstrap/Table'
import Badge from 'react-bootstrap/Badge'
import type { Flow } from '../utils/flow'

interface IProps {
  flows: Flow[]
}

function Statistics({ flows }: IProps) {
  const totalFlows = flows.length
  const statusCodes: Record<string, number> = {}
  let totalBytesSent = 0
  let totalBytesReceived = 0
  const totalResponseTime = 0
  let responseCount = 0

  flows.forEach(f => {
    if (f.response) {
      const code = String(f.response.statusCode)
      statusCodes[code] = (statusCodes[code] || 0) + 1

      if (f.response.body) {
        totalBytesReceived += f.response.body.byteLength
      }
      responseCount++
    }
    if (f.request?.body) {
      totalBytesSent += f.request.body.byteLength
    }
  })

  const avgResponseMs = responseCount > 0 ? Math.round(totalResponseTime / responseCount) : 0

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
  }

  // Group status codes by category
  const categories: Record<string, { count: number; variant: string }> = {
    '2xx': { count: 0, variant: 'success' },
    '3xx': { count: 0, variant: 'info' },
    '4xx': { count: 0, variant: 'warning' },
    '5xx': { count: 0, variant: 'danger' },
    'other': { count: 0, variant: 'secondary' },
  }

  Object.entries(statusCodes).forEach(([code, count]) => {
    const c = parseInt(code)
    if (c >= 200 && c < 300) categories['2xx'].count += count
    else if (c >= 300 && c < 400) categories['3xx'].count += count
    else if (c >= 400 && c < 500) categories['4xx'].count += count
    else if (c >= 500 && c < 600) categories['5xx'].count += count
    else categories['other'].count += count
  })

  return (
    <div style={{ padding: '20px' }}>
      <h6>Traffic Statistics</h6>
      <Table bordered size="sm" style={{ maxWidth: '400px' }}>
        <tbody>
          <tr><td>Total Requests</td><td><strong>{totalFlows}</strong></td></tr>
          <tr><td>Data Sent</td><td>{formatBytes(totalBytesSent)}</td></tr>
          <tr><td>Data Received</td><td>{formatBytes(totalBytesReceived)}</td></tr>
          {avgResponseMs > 0 && <tr><td>Avg Response Time</td><td>{avgResponseMs} ms</td></tr>}
        </tbody>
      </Table>

      <h6 style={{ marginTop: '20px' }}>Status Code Distribution</h6>
      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
        {Object.entries(categories).map(([cat, { count, variant }]) => {
          if (count === 0) return null
          return (
            <Badge key={cat} bg={variant} style={{ fontSize: '14px', padding: '6px 12px' }}>
              {cat}: {count}
            </Badge>
          )
        })}
      </div>

      {Object.keys(statusCodes).length > 0 && (
        <Table striped bordered size="sm" style={{ maxWidth: '300px', marginTop: '15px' }}>
          <thead>
            <tr><th>Status Code</th><th>Count</th></tr>
          </thead>
          <tbody>
            {Object.entries(statusCodes).sort(([a], [b]) => parseInt(a) - parseInt(b)).map(([code, count]) => (
              <tr key={code}><td>{code}</td><td>{count}</td></tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  )
}

export default Statistics

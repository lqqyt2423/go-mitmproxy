import React from 'react'
import type { ITimingData } from '../utils/flow'

interface IProps {
  timing: ITimingData
}

const PHASES = [
  { key: 'dnsMs', label: 'DNS Lookup', color: '#28a745' },
  { key: 'connectMs', label: 'TCP Connect', color: '#17a2b8' },
  { key: 'tlsMs', label: 'TLS Handshake', color: '#6f42c1' },
  { key: 'sendMs', label: 'Request Sent', color: '#fd7e14' },
  { key: 'waitMs', label: 'Waiting (TTFB)', color: '#007bff' },
  { key: 'receiveMs', label: 'Content Download', color: '#dc3545' },
] as const

function Timing({ timing }: IProps) {
  const total = PHASES.reduce((sum, p) => sum + (timing[p.key] || 0), 0)
  const maxWidth = 300

  return (
    <div>
      <table style={{ borderCollapse: 'collapse', width: '100%', maxWidth: '600px' }}>
        <thead>
          <tr>
            <th style={{ padding: '4px 8px', textAlign: 'left', width: '140px' }}>Phase</th>
            <th style={{ padding: '4px 8px', textAlign: 'left' }}>Duration</th>
            <th style={{ padding: '4px 8px', textAlign: 'right', width: '60px' }}>ms</th>
          </tr>
        </thead>
        <tbody>
          {PHASES.map(phase => {
            const value = timing[phase.key] || 0
            const width = total > 0 ? Math.max(2, (value / total) * maxWidth) : 0
            return (
              <tr key={phase.key}>
                <td style={{ padding: '4px 8px', fontSize: '13px' }}>{phase.label}</td>
                <td style={{ padding: '4px 8px' }}>
                  <div style={{
                    width: `${width}px`,
                    height: '16px',
                    backgroundColor: phase.color,
                    borderRadius: '2px',
                    minWidth: value > 0 ? '2px' : '0',
                  }} />
                </td>
                <td style={{ padding: '4px 8px', textAlign: 'right', fontSize: '13px', fontFamily: 'monospace' }}>
                  {value}
                </td>
              </tr>
            )
          })}
          <tr style={{ borderTop: '1px solid #dee2e6', fontWeight: 'bold' }}>
            <td style={{ padding: '4px 8px' }}>Total</td>
            <td></td>
            <td style={{ padding: '4px 8px', textAlign: 'right', fontFamily: 'monospace' }}>{total}</td>
          </tr>
        </tbody>
      </table>
    </div>
  )
}

export default Timing

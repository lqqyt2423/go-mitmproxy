import React from 'react'
import { createRoot } from 'react-dom/client'
import 'bootstrap/dist/css/bootstrap.min.css'
import App from './App'

const root = createRoot(document.getElementById('root') as HTMLElement)
root.render(<App />)

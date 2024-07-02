import { Dispatch, SetStateAction, useState } from 'react'

interface IConfig<S> {
  get: () => S,
  set: (v: S) => void,
}

export function useConfig<S>(config: IConfig<S>): [S, Dispatch<SetStateAction<S>>] {
  const [initialValue, setValue] = useState(config.get())

  const setConfig: Dispatch<SetStateAction<S>> = (v) => {
    setValue(v)
    if (typeof v !== 'function') {
      config.set(v)
    }
  }

  return [initialValue, setConfig]
}

export const configViewFlowTab = (() => {
  type Value = 'Headers' | 'Preview' | 'Response' | 'Hexview' | 'Detail'
  const key = 'go-mitm.configViewFlowTab'
  return {
    get: () => (localStorage.getItem(key) || 'Detail') as Value,
    set: (value: Value) => localStorage.setItem(key, value),
  }
})()

export const configViewFlowResponseBodyLineBreak = (() => {
  const key = 'go-mitm.configViewFlowResponseBodyLineBreak'
  return {
    get: () => (localStorage.getItem(key) || 'false') === 'true',
    set: (value: boolean) => localStorage.setItem(key, value ? 'true' : 'false'),
  }
})()

export const configViewFlowRequestBodyTab = (() => {
  type Value = 'Raw' | 'Preview'
  const key = 'go-mitm.configViewFlowRequestBodyTab'
  return {
    get: () => (localStorage.getItem(key) || 'Raw') as Value,
    set: (value: Value) => localStorage.setItem(key, value),
  }
})()

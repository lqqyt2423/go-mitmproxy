import { Flow } from './message'

export class FlowManager {
  private items: Flow[]
  private _map: Map<string, Flow>
  private filterText: string
  private filterTimer: number | null
  private num: number
  private max: number

  constructor() {
    this.items = []
    this._map = new Map()
    this.filterText = ''
    this.filterTimer = null
    this.num = 0

    this.max = 1000
  }

  showList() {
    let text = this.filterText
    if (text) text = text.trim()
    if (!text) return this.items

    // regexp
    if (text.startsWith('/') && text.endsWith('/')) {
      text = text.slice(1, text.length - 1).trim()
      if (!text) return this.items
      try {
        const reg = new RegExp(text)
        return this.items.filter(item => {
          return reg.test(item.request.url)
        })
      } catch (err) {
        return this.items
      }
    }

    return this.items.filter(item => {
      return item.request.url.includes(text)
    })
  }

  add(item: Flow) {
    item.no = ++this.num
    this.items.push(item)
    this._map.set(item.id, item)

    if (this.items.length > this.max) {
      const oldest = this.items.shift()
      if (oldest) this._map.delete(oldest.id)
    }
  }

  get(id: string) {
    return this._map.get(id)
  }

  changeFilter(text: string) {
    this.filterText = text
  }

  changeFilterLazy(text: string, callback: () => void) {
    if (this.filterTimer) {
      clearTimeout(this.filterTimer)
      this.filterTimer = null
    }

    this.filterTimer = setTimeout(() => {
      this.filterText = text
      callback()
    }, 300) as any
  }

  clear() {
    this.items = []
    this._map = new Map()
  }
}

export class FlowManager {
  constructor() {
    this.items = []
    this._map = new Map()
    this.filterText = ''
    this.filterTimer = null
    this.num = 0

    this.max = 1000
  }

  showList() {
    if (!this.filterText) return this.items
    return this.items.filter(item => {
      return item.request.url.includes(this.filterText)
    })
  }

  add(item) {
    item.no = ++this.num
    this.items.push(item)
    this._map.set(item.id, item)
    
    if (this.items.length > this.max) {
      const oldest = this.items.shift()
      this._map.delete(oldest.id)
    }
  }

  get(id) {
    return this._map.get(id)
  }

  changeFilter(text) {
    this.filterText = text
  }

  changeFilterLazy(text, callback) {
    if (this.filterTimer) {
      clearTimeout(this.filterTimer)
      this.filterTimer = null
    }

    this.filterTimer = setTimeout(() => {
      this.filterText = text
      callback()
    }, 300)
  }

  clear() {
    this.items = []
    this._map = new Map()
  }
}

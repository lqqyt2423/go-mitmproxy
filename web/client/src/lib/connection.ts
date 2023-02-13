export interface IConnection {
  clientConn: {
    id: string
    tls: boolean
    address: string
  }
  serverConn?: {
    id: string
    address: string
    peername: string
  }
  intercept: boolean
  opening?: boolean
  flowCount?: number
}

export class ConnectionManager {
  private _map: Map<string, IConnection>

  constructor() {
    this._map = new Map()
  }

  get(id: string) {
    return this._map.get(id)
  }

  add(id: string, conn: IConnection) {
    this._map.set(id, conn)
  }

  delete(id: string) {
    this._map.delete(id)
  }
}

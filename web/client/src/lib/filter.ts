import type { Flow, Header } from './flow'
import filterRuleParser from './filterRuleParser'

const FLOW_FILTER_SCOPES = ['url', 'method', 'code', 'header', 'reqheader', 'resheader', 'body', 'reqbody', 'resbody', 'all'] as const
type FlowFilterScope = typeof FLOW_FILTER_SCOPES[number]

type Rule = IRuleKeyword | IRuleNot | IRuleAnd | IRuleOr

interface IRuleKeyword {
  type: 'keyword'
  value: string
  filter?: FlowFilterKeyword
}

interface IRuleNot {
  type: 'not'
  expr: Rule
}

interface IRuleAnd {
  type: 'and'
  left: Rule
  right: Rule
}

interface IRuleOr {
  type: 'or'
  left: Rule
  right: Rule
}

export class FlowFilter {
  private rule: Rule | undefined
  constructor(text: string) {
    text = text.trim()
    if (!text) return
    this.rule = filterRuleParser.parse(text)
  }

  public match(flow: Flow): boolean {
    if (!this.rule) return true
    return this._match(flow, this.rule)
  }

  private _match(flow: Flow, rule: Rule): boolean {
    if (rule.type === 'keyword') {
      return this.getFlowFilterKeyword(rule).match(flow)
    }
    else if (rule.type === 'not') {
      return !this._match(flow, rule.expr)
    }
    else if (rule.type === 'and') {
      return this._match(flow, rule.left) && this._match(flow, rule.right)
    }
    else if (rule.type === 'or') {
      return this._match(flow, rule.left) || this._match(flow, rule.right)
    }
    else {
      // eslint-disable-next-line
      // @ts-ignore
      throw new Error(`invalid rule type ${rule.type}`)
    }
  }

  private getFlowFilterKeyword(rule: IRuleKeyword): FlowFilterKeyword {
    if (rule.filter) return rule.filter
    rule.filter = new FlowFilterKeyword(rule.value)
    return rule.filter
  }
}

export class FlowFilterKeyword {
  private keyword: string | RegExp | undefined
  private scope: FlowFilterScope = 'url'

  constructor(text: string) {
    text = text.trim()
    if (!text) return

    for (const scope of FLOW_FILTER_SCOPES) {
      if (text.startsWith(`${scope}:`)) {
        this.scope = scope
        text = text.replace(`${scope}:`, '').trim()
        break
      }
    }
    if (!text) return

    // regexp
    if (text.startsWith('/') && (text.endsWith('/') || text.endsWith('/i'))) {
      let flags: string | undefined
      if (text.endsWith('i')) {
        flags = 'i'
        text = text.slice(0, -1)
      }
      text = text.slice(1, -1).trim()
      if (!text) return
      this.keyword = new RegExp(text, flags)
    }
    // string
    else {
      this.keyword = text
    }
  }

  public match(flow: Flow): boolean {
    switch (this.scope) {
    case 'url':
      return this.matchUrl(flow)
    case 'method':
      return this.matchMethod(flow)
    case 'code':
      return this.matchCode(flow)
    case 'reqheader':
      return this.matchReqHeader(flow)
    case 'resheader':
      return this.matchResHeader(flow)
    case 'header':
      return this.matchHeader(flow)
    case 'reqbody':
      return this.matchReqBody(flow)
    case 'resbody':
      return this.matchResBody(flow)
    case 'body':
      return this.matchBody(flow)
    case 'all':
      return this.matchAll(flow)
    default:
      throw new Error(`invalid scope ${this.scope}`)
    }
  }

  private matchUrl(flow: Flow): boolean {
    return this.matchKeyword(flow.request.url)
  }

  private matchMethod(flow: Flow): boolean {
    return this.matchKeyword(flow.request.method) || this.matchKeyword(flow.request.method.toLowerCase())
  }

  private matchCode(flow: Flow): boolean {
    if (!flow.response) return false
    return this.matchKeyword(flow.response.statusCode.toString())
  }

  private _matchHeader(header: Header): boolean {
    return Object.entries(header).some(([key, vals]) => {
      return [key].concat(vals).some(text => this.matchKeyword(text))
    })
  }

  private matchReqHeader(flow: Flow): boolean {
    return this._matchHeader(flow.request.header)
  }

  private matchResHeader(flow: Flow): boolean {
    if (!flow.response) return false
    return this._matchHeader(flow.response.header)
  }

  private matchHeader(flow: Flow): boolean {
    return this.matchReqHeader(flow) || this.matchResHeader(flow)
  }

  private matchReqBody(flow: Flow): boolean {
    const body = flow.requestBody()
    if (!body) return false
    return this.matchKeyword(body)
  }

  private matchResBody(flow: Flow): boolean {
    const body = flow.responseBody()
    if (!body) return false
    return this.matchKeyword(body)
  }

  private matchBody(flow: Flow): boolean {
    return this.matchReqBody(flow) || this.matchResBody(flow)
  }

  private matchAll(flow: Flow): boolean {
    return this.matchUrl(flow) || this.matchMethod(flow) || this.matchHeader(flow) || this.matchBody(flow)
  }

  private matchKeyword(text: string): boolean {
    if (!this.keyword) return true
    if (!text) return false
    if (this.keyword instanceof RegExp) return this.keyword.test(text)
    return text.includes(this.keyword)
  }
}

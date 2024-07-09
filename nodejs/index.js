'use strict';

const { createTSFN, closeMitmProxy, cAckMessage } = require('bindings')('ngmp_addon');
const onChange = require('on-change');

const createMitmProxy = () => new MitmProxy();

class MitmProxy {
  constructor() {
    this.visitors = [];
  }

  start() {
    const flowVisitor = ['hookRequestheaders', 'hookRequest', 'hookResponseheaders', 'hookResponse'].reduce((res, hookAt) => {
      const fns = this.visitors.map((visitor) => visitor[hookAt]).filter((fn) => !!fn && typeof fn === 'function');
      if (fns.length) {
        res[hookAt] = async (flow) => {
          for (const fn of fns) {
            const resFlow = await fn(flow);
            if (resFlow != null) flow = resFlow;
          }
          return flow;
        };
      }
      return res;
    }, {});

    newGoMitmProxy(flowVisitor);
    return this;
  }

  close() {
    closeMitmProxy();
  }

  registerCloseSignal() {
    process.on('SIGINT', () => {
      this.close();
    });

    process.on('SIGTERM', () => {
      this.close();
    });
  }

  /**
   *
   * @param {import("./types").FlowVisitor} visitor
   */
  addAddon(visitor) {
    this.visitors.push(visitor);
    return this;
  }
}

/**
 *
 * @param {import("./types").FlowVisitor} flowVisitor
 */
const newGoMitmProxy = async function (flowVisitor = {}) {
  const onMessage = (msg) => {
    let payload;
    try {
      payload = JSON.parse(msg);
    } catch (err) {
      return;
    }

    const hookAt = payload.hookAt;
    const ackMessageNoChange = () => ackMessage(hookAt, 'noChange', payload.flow);
    const ackMessageChange = (flow) => ackMessage(hookAt, 'change', flow);

    const handler = flowVisitor[`hook${hookAt}`];
    if (!handler) {
      ackMessageNoChange();
      return;
    }

    Object.setPrototypeOf(payload.flow.request, reqOrResProto);
    if (payload.flow.response) {
      Object.setPrototypeOf(payload.flow.response, reqOrResProto);
    }

    payload.flow.request.header = transformHeader(payload.flow.request.header);
    if (payload.flow.response) {
      payload.flow.response.header = transformHeader(payload.flow.response.header);
    }

    if (payload.flow.request.body != null) {
      payload.flow.request.body = Buffer.from(payload.flow.request.body, 'base64');
    }
    if (payload.flow.response?.body != null) {
      payload.flow.response.body = Buffer.from(payload.flow.response.body, 'base64');
    }

    const rawId = payload.flow.id;
    let dirty = false;

    let flow = new Proxy(payload.flow, {});
    flow.request = new Proxy(flow.request, {
      set(target, property, value, receiver) {
        dirty = true;
        return Reflect.set(...arguments);
      },
    });
    if (flow.response) {
      flow.response = new Proxy(flow.response, {
        set(target, property, value, receiver) {
          dirty = true;
          return Reflect.set(...arguments);
        },
      });
    }
    flow = onChange(
      flow,
      function (path, value, previousValue, name) {
        dirty = true;
      },
      // Buffer类型会报错，body是Buffer类型，所以才会有上面的额外Proxy的部分
      { ignoreKeys: ['body'] }
    );

    Promise.resolve()
      .then(() => handler(flow))
      .then((mayChangedFlow) => {
        if (!mayChangedFlow) mayChangedFlow = flow;

        if (mayChangedFlow.id !== rawId) {
          ackMessageNoChange();
          return;
        }

        if (mayChangedFlow !== flow) {
          ackMessageChange(mayChangedFlow);
          return;
        }

        if (dirty) {
          ackMessageChange(payload.flow);
        } else {
          ackMessageNoChange();
        }
      })
      .catch((err) => {
        console.error(err);
      });
  };

  await createTSFN(onMessage);
};

const ackMessage = (hookAt, action, flow) => {
  const am = {
    action,
    hookAt,
    id: flow.id,
    flow: action === 'change' ? flow : null,
  };

  if (am.flow) {
    am.flow.request.header = transformHeaderBack(am.flow.request.header);
  }
  if (am.flow?.response) {
    am.flow.response.header = transformHeaderBack(am.flow.response.header);
  }

  if (am.flow?.request.body != null && Buffer.isBuffer(am.flow.request.body)) {
    am.flow.request.body = am.flow.request.body.toString('base64');
  }
  if (am.flow?.response?.body != null && Buffer.isBuffer(am.flow.response.body)) {
    am.flow.response.body = am.flow.response.body.toString('base64');
  }

  cAckMessage(JSON.stringify(am));
};

const transformHeader = (header) => {
  return Object.keys(header).reduce((res, key) => {
    res[key.toLowerCase()] = header[key].length > 1 ? header[key] : header[key][0];
    return res;
  }, {});
};

const transformHeaderBack = (header) => {
  return Object.keys(header).reduce((res, key) => {
    res[key] = Array.isArray(header[key]) ? header[key] : [header[key]];
    return res;
  }, {});
};

const reqOrResProto = {
  setBody(body) {
    if (typeof body === 'string') body = Buffer.from(body);
    this.header['content-length'] = body.length.toString();
    this.body = body;
  },
};

module.exports = {
  createMitmProxy,
};

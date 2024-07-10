'use strict';

const { createTSFN, closeMitmProxy, cAckMessage } = require('bindings')('ngmp_addon');

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
    const rawFlow = payload.flow;
    const rawFlowId = rawFlow.id;
    /** @type {'change' | 'noChange'} */
    let action = 'noChange';
    let ackFlow;

    // use for flowToNode and flowToGo
    const ctx = { requestHeaderKeysMap: {}, responseHeaderKeysMap: {} };

    const ackMessage = () => {
      if (ackFlow && ackFlow.id !== rawFlowId) ackFlow = null;
      if (!ackFlow) action = 'noChange';
      if (action === 'noChange' && ackFlow) ackFlow = null;

      if (ackFlow) flowToGo(ackFlow, ctx);

      const am = {
        action,
        hookAt,
        id: rawFlowId,
        flow: ackFlow,
      };
      // console.log(am);
      cAckMessage(JSON.stringify(am));
    };

    const handler = flowVisitor[`hook${hookAt}`];
    if (!handler) return ackMessage();

    flowToNode(rawFlow, ctx);

    const watch = makeWatch(rawFlow);
    Promise.resolve()
      .then(() => handler(watch.flow))
      .then((resFlow) => {
        if ((resFlow == null || resFlow == watch.flow) && watch.isDirty()) {
          action = 'change';
          ackFlow = rawFlow;
        }
        if (resFlow != null && resFlow != watch.flow) {
          action = 'change';
          ackFlow = resFlow;
        }
      })
      .catch((err) => {
        console.error(err);
      })
      .then(watch.markEnd)
      .then(ackMessage);
  };

  await createTSFN(onMessage);
};

const makeWatch = (flow) => {
  let dirty = false;
  let end = false;

  const proxyHeader = (header) => {
    return new Proxy(header, {
      get(target, property, receiver) {
        if (end) return Reflect.get(...arguments);

        if (typeof property === 'symbol') return Reflect.get(...arguments);
        property = property.toLowerCase();
        return Reflect.get(target, property, receiver);
      },
      set(target, property, value, receiver) {
        if (end) return Reflect.set(...arguments);

        dirty = true;
        if (typeof property === 'symbol') return Reflect.set(...arguments);

        property = property.toLowerCase();
        return Reflect.set(target, property, value, receiver);
      },
      deleteProperty(target, property) {
        if (end) return Reflect.deleteProperty(...arguments);

        dirty = true;
        if (typeof property === 'symbol') return Reflect.deleteProperty(...arguments);
        property = property.toLowerCase();
        return Reflect.deleteProperty(target, property);
      },
    });
  };

  const proxyReqOrRes = (obj) => {
    return new Proxy(obj, {
      set(target, property, value, receiver) {
        if (end) return Reflect.set(...arguments);

        dirty = true;
        if (typeof property === 'symbol') return Reflect.set(...arguments);
        if (property !== 'body') return Reflect.set(...arguments);

        // body
        if (typeof value === 'string') value = Buffer.from(value);
        target.header['content-length'] = value.length.toString();
        return Reflect.set(target, property, value, receiver);
      },
    });
  };

  flow.request.header = proxyHeader(flow.request.header);
  if (flow.response) flow.response.header = proxyHeader(flow.response.header);

  flow.request = proxyReqOrRes(flow.request);
  if (flow.response) flow.response = proxyReqOrRes(flow.response);

  return {
    flow,
    isDirty: () => dirty,
    markEnd: () => {
      end = true;
    },
  };
};

const flowToNode = (flow, ctx) => {
  const transformHeader = (header, map) => {
    return Object.keys(header).reduce((res, key) => {
      const value = header[key];
      const newKey = key.toLowerCase();
      map[newKey] = key;
      res[newKey] = value.length > 1 ? value : value[0];
      return res;
    }, {});
  };

  flow.request.header = transformHeader(flow.request.header, ctx.requestHeaderKeysMap);
  if (flow.response) {
    flow.response.header = transformHeader(flow.response.header, ctx.responseHeaderKeysMap);
  }

  if (flow.request.body != null) {
    flow.request.body = Buffer.from(flow.request.body, 'base64');
  }
  if (flow.response?.body != null) {
    flow.response.body = Buffer.from(flow.response.body, 'base64');
  }
};

const flowToGo = (flow, ctx) => {
  const transformHeaderBack = (header, map) => {
    return Object.keys(header).reduce((res, key) => {
      const realKey = map[key] || key;
      const value = header[key];
      res[realKey] = Array.isArray(value) ? value : [value];
      return res;
    }, {});
  };

  flow.request.header = transformHeaderBack(flow.request.header, ctx.requestHeaderKeysMap);
  if (flow.response) {
    flow.response.header = transformHeaderBack(flow.response.header, ctx.responseHeaderKeysMap);
  }
  if (flow.request.body != null && Buffer.isBuffer(flow.request.body)) {
    flow.request.body = flow.request.body.toString('base64');
  }
  if (flow.response?.body != null && Buffer.isBuffer(flow.response.body)) {
    flow.response.body = flow.response.body.toString('base64');
  }
};

module.exports = {
  newGoMitmProxy,
  closeMitmProxy,
};

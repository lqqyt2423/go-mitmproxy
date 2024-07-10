'use strict';

const { createTSFN, closeMitmProxy, cAckMessage } = require('bindings')('ngmp_addon');
const onChange = require('on-change');

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
      .then(ackMessage);
  };

  await createTSFN(onMessage);
};

const reqOrResProto = {
  setBody(body) {
    if (typeof body === 'string') body = Buffer.from(body);
    this.header['content-length'] = body.length.toString();
    this.body = body;
  },
};

const makeWatch = (rawFlow) => {
  let dirty = false;

  let flow = new Proxy(rawFlow, {});
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

  return {
    flow,
    isDirty: () => dirty,
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

  Object.setPrototypeOf(flow.request, reqOrResProto);
  if (flow.response) {
    Object.setPrototypeOf(flow.response, reqOrResProto);
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

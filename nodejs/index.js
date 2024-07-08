'use strict';

const { createTSFN, closeMitmProxy, cAckMessage } = require('bindings')('ngmp_addon');
const onChange = require('on-change');

const ackMessage = (hookAt, action, flow) => {
  const am = {
    action,
    hookAt,
    id: flow.id,
    flow: action === 'change' ? flow : null,
  };
  if (am.flow?.request.body != null && Buffer.isBuffer(am.flow.request.body)) {
    am.flow.request.body = am.flow.request.body.toString('base64');
  }
  if (am.flow?.response?.body != null && Buffer.isBuffer(am.flow.response.body)) {
    am.flow.response.body = am.flow.response.body.toString('base64');
  }
  cAckMessage(JSON.stringify(am));
};

const newMitmProxy = async function (handlers = {}) {
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

    const handler = handlers[`hook${hookAt}`];
    if (!handler) {
      ackMessageNoChange();
      return;
    }

    if (payload.flow.request.body != null) {
      payload.flow.request.body = Buffer.from(payload.flow.request.body, 'base64');
    }
    if (payload.flow.response?.body != null) {
      payload.flow.response.body = Buffer.from(payload.flow.response.body, 'base64');
    }

    const rawId = payload.flow.id;
    let dirty = false;
    // todo: change this pkg
    const flow = onChange(payload.flow, function (path, value, previousValue, name) {
      dirty = true;
    });

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

module.exports = {
  newMitmProxy,
  closeMitmProxy,
};

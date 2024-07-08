'use strict';

const { createTSFN, closeMitmProxy } = require('bindings')('ngmp_addon');
const onChange = require('on-change');

// todo: add golang method
const ackMessage = (hookAt, action, payload) => {
  console.log('ackMessage', hookAt, action, payload?.id);
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
    const ackMessageNoChange = () => ackMessage(hookAt, 'noChange', null);
    const ackMessageChange = (payload) => ackMessage(hookAt, 'change', payload);

    const handler = handlers[`hook${hookAt}`];
    if (!handler) {
      ackMessageNoChange();
      return;
    }

    const rawId = payload.flow.id;
    let dirty = false;
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
          ackMessageChange(flow);
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

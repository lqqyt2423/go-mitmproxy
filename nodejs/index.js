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

    const handler = handlers[`hook${payload.hookAt}`];
    if (!handler) return;

    const rawId = payload.flow.id;
    payload.flow._dirty = false;
    const flow = onChange(payload.flow, function (path, value, previousValue, name) {
      payload.flow._dirty = true;
    });

    Promise.resolve()
      .then(() => handler(flow))
      .then((mayChangedFlow) => {
        mayChangedFlow = mayChangedFlow || flow;
        if (mayChangedFlow.id !== rawId) {
          ackMessage(payload.hookAt, 'noChange', null);
          return;
        }

        if (mayChangedFlow._dirty === true || mayChangedFlow._dirty == null) {
          ackMessage(payload.hookAt, 'change', mayChangedFlow);
        } else {
          ackMessage(payload.hookAt, 'noChange', null);
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

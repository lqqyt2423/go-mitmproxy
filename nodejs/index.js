'use strict';

const { createTSFN } = require('bindings')('ngmp_addon');

module.exports = async function (handlers = {}) {
  const onMessage = (msg) => {
    let payload;
    try {
      payload = JSON.parse(msg);
    } catch (err) {
      return;
    }

    const handler = handlers[`hook${payload.hookAt}`];
    if (!handler) return;

    const flow = new Proxy(payload.flow, {
      set(target, property, value, receiver) {
        console.log(`setting ${property}=${value}`);
        return Reflect.set(...arguments);
      },
    });

    Promise.resolve()
      .then(() => handler(flow))
      .then((res) => {
        console.log('ok');
        console.log(res);
      })
      .catch((err) => {
        console.error(err);
      });
  };

  await createTSFN(onMessage);
};

'use strict';

const { newMitmProxy, closeMitmProxy } = require('./index');

newMitmProxy({
  hookRequestheaders: async (flow) => {
    console.log('in hookRequestheaders', flow);
  },
});

process.on('SIGINT', () => {
  closeMitmProxy();
});

process.on('SIGTERM', () => {
  closeMitmProxy();
});

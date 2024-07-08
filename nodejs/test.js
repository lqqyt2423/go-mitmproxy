'use strict';

const { newMitmProxy, closeMitmProxy } = require('./index');

newMitmProxy({
  hookRequestheaders: async (flow) => {
    // console.log('in hookRequestheaders', flow);
  },
  hookRequest: async (flow) => {
    // console.log('in hookRequest', flow);
  },
  hookResponseheaders: async (flow) => {
    // console.log('in hookResponseheaders', flow);
  },
  hookResponse: async (flow) => {
    console.log('in hookResponse', flow);
    flow.response.body = Buffer.from('hello world');
    flow.response.header['Content-Length'] = [flow.response.body.length.toString()];
  },
});

process.on('SIGINT', () => {
  closeMitmProxy();
});

process.on('SIGTERM', () => {
  closeMitmProxy();
});

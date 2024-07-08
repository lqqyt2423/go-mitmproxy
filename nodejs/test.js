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
    const body = Buffer.from('hello world');
    const bodyLen = body.length.toString();
    flow.response.body = body;
    flow.response.header['Content-Length'] = [bodyLen];
  },
});

process.on('SIGINT', () => {
  closeMitmProxy();
});

process.on('SIGTERM', () => {
  closeMitmProxy();
});

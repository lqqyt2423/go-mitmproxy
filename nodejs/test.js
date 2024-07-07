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
    // if (flow.response.body) {
    //   console.log('body', Buffer.from(flow.response.body, 'base64').toString());
    // }
  },
});

process.on('SIGINT', () => {
  closeMitmProxy();
});

process.on('SIGTERM', () => {
  closeMitmProxy();
});

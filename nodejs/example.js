'use strict';

const { createMitmProxy } = require('./index');

createMitmProxy()
  .addAddon({
    hookRequestheaders: async (flow) => {
      console.log('in hookRequestheaders', flow);
    },
    hookRequest: async (flow) => {
      console.log('in hookRequest', flow);
    },
    hookResponseheaders: async (flow) => {
      console.log('in hookResponseheaders', flow);
    },
    hookResponse: async (flow) => {
      console.log('in hookResponse', flow);
      flow.response.body = 'hello world';
    },
  })
  .start()
  .registerCloseSignal();

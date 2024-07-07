'use strict';

const ngmp = require('.');

console.log('pid', process.pid);

ngmp({
  hookRequestheaders: async (flow) => {
    console.log('in hookRequestheaders', flow);
  },
});

// keep alive
setInterval(() => {
  console.log(new Date(), 'in setInterval');
}, 1000 * 3);

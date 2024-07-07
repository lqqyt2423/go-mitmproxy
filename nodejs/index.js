const { createTSFN } = require('bindings')('ngmp_addon');

const callback = (...args) => {
  console.log(new Date(), ...args);
};

module.exports = async function () {
  console.log(await createTSFN(callback));
};

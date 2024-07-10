'use strict';

const { newGoMitmProxy, closeMitmProxy } = require('./bridge');

const createMitmProxy = () => new MitmProxy();

class MitmProxy {
  constructor() {
    this.visitors = [];
  }

  start() {
    const flowVisitor = ['hookRequestheaders', 'hookRequest', 'hookResponseheaders', 'hookResponse'].reduce((res, hookAt) => {
      const fns = this.visitors.map((visitor) => visitor[hookAt]).filter((fn) => !!fn && typeof fn === 'function');
      if (fns.length) {
        res[hookAt] = async (flow) => {
          for (const fn of fns) {
            const resFlow = await fn(flow);
            if (resFlow != null) flow = resFlow;
          }
          return flow;
        };
      }
      return res;
    }, {});

    newGoMitmProxy(flowVisitor);
    return this;
  }

  close() {
    closeMitmProxy();
  }

  registerCloseSignal() {
    process.on('SIGINT', () => {
      this.close();
    });

    process.on('SIGTERM', () => {
      this.close();
    });
  }

  /**
   *
   * @param {import("./types").FlowVisitor} visitor
   */
  addAddon(visitor) {
    this.visitors.push(visitor);
    return this;
  }
}

module.exports = {
  createMitmProxy,
};

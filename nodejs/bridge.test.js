'use strict';

jest.mock('bindings');
const bindings = require('bindings');
const EventEmitter = require('events');

let emitMessage;
const ackEmitter = new EventEmitter();

bindings.mockImplementation(() => ({
  createTSFN: (fn) => {
    emitMessage = (msg) => fn(JSON.stringify(msg));
  },
  closeMitmProxy: () => {},
  cAckMessage: (data) => {
    ackEmitter.emit('data', JSON.parse(data));
  },
}));

const sendThenWait = (msg) => {
  return new Promise((resolve, reject) => {
    ackEmitter.once('data', resolve);
    emitMessage(msg);
  });
};

const { newGoMitmProxy } = require('./bridge');

const msg = {
  hookAt: 'Response',
  flow: {
    id: '123',
    request: {
      method: 'GET',
      header: {
        'User-Agent': ['curl/8.6.0'],
      },
      body: '',
    },
    response: {
      statusCode: 200,
      header: {
        'Content-Length': ['11'],
        'Content-Type': ['text/html; charset=UTF-8'],
        'Set-Cookie': ['a=1; path=/', ' b=1; path=/'],
      },
      body: 'aGVsbG8gd29ybGQ=', // hello world
    },
  },
};

test('should ack noChange when no hooks', async () => {
  await newGoMitmProxy({});
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('noChange');
});

test('should ack noChange when hook not change flow', async () => {
  await newGoMitmProxy({
    hookResponse() {},
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('noChange');
});

test('in hook header key should be lowercase', async () => {
  let a, b;
  await newGoMitmProxy({
    hookResponse(flow) {
      a = Object.keys(flow.request.header);
      b = Object.keys(flow.response.header);
    },
  });
  await sendThenWait(msg);
  expect(a).toStrictEqual(['user-agent']);
  expect(b).toStrictEqual(['content-length', 'content-type', 'set-cookie']);
});

test('in hook header can get key case insensitive', async () => {
  let a, b, c, d;
  await newGoMitmProxy({
    hookResponse(flow) {
      a = flow.request.header['User-Agent'];
      b = flow.request.header['user-agent'];
      c = flow.response.header['Content-Length'];
      d = flow.response.header['content-length'];
    },
  });
  await sendThenWait(msg);
  expect(a).toBe('curl/8.6.0');
  expect(b).toBe('curl/8.6.0');
  expect(c).toBe('11');
  expect(d).toBe('11');
});

test('in hook header can set key case insensitive', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      flow.request.header['user-agent'] = 'test-agent';
      flow.response.header['Content-Type'] = 'text/plain';
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.request.header['User-Agent']).toStrictEqual(['test-agent']);
  expect(ack.flow.response.header['Content-Type']).toStrictEqual(['text/plain']);
});

test('in hook header value should be string if array len is 1', async () => {
  let a, b, c;
  await newGoMitmProxy({
    hookResponse(flow) {
      a = flow.request.header['user-agent'];
      b = flow.response.header['content-length'];
      c = flow.response.header['set-cookie'];
    },
  });
  await sendThenWait(msg);
  expect(a).toBe('curl/8.6.0');
  expect(b).toBe('11');
  expect(c).toStrictEqual(['a=1; path=/', ' b=1; path=/']);
});

test('ack should be change when flow changed', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      flow.response.header['content-type'] = 'text/plain';
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['Content-Type']).toStrictEqual(['text/plain']);
});

test('can add header', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      flow.response.header['x-count'] = '1';
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['x-count']).toStrictEqual(['1']);
});

test('can del header lowercase', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      delete flow.response.header['content-type'];
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['Content-Type']).toBeUndefined();
});

test('can del header', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      delete flow.response.header['Content-Type'];
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['Content-Type']).toBeUndefined();
});

test('can change body to Buffer', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      flow.response.body = Buffer.from('hello');
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['Content-Length']).toStrictEqual(['5']);
  expect(ack.flow.response.body).toBe(Buffer.from('hello').toString('base64'));
});

test('can change body to string', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      flow.response.body = 'hello';
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['Content-Length']).toStrictEqual(['5']);
  expect(ack.flow.response.body).toBe(Buffer.from('hello').toString('base64'));
});

test('can change body to empty', async () => {
  await newGoMitmProxy({
    hookResponse(flow) {
      flow.response.body = '';
    },
  });
  const ack = await sendThenWait(msg);
  expect(ack.action).toBe('change');
  expect(ack.flow.response.header['Content-Length']).toStrictEqual(['0']);
  expect(ack.flow.response.body).toBe('');
});

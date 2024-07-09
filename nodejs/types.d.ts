import { IncomingHttpHeaders } from 'http';

type Header = IncomingHttpHeaders;

interface Request {
  method: string;
  url: string;
  proto: string;
  header: Header;
  body?: Buffer;
  
  setBody: (body: Buffer) => void;
}

interface Response {
  statusCode: number;
  header: Header;
  body?: Buffer;

  setBody: (body: Buffer) => void;
}

interface Flow {
  id: string;
  request: Request;
  response?: Response;
}

type Handler = (flow: Flow) => void | Flow | Promise<void | Flow>

export interface Handlers {
  hookRequestheaders?: Handler;
  hookRequest?: Handler;
  hookResponseheaders?: Handler;
  hookResponse?: Handler;
}

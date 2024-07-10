import { IncomingHttpHeaders } from 'http';

type Header = IncomingHttpHeaders;

interface Request {
  method: string;
  url: string;
  proto: string;
  header: Header;
  body?: Buffer;
}

interface Response {
  statusCode: number;
  header: Header;
  body?: Buffer;
}

interface Flow {
  id: string;
  request: Request;
  response?: Response;
}

type Handler = (flow: Flow) => void | Flow | Promise<void | Flow>

export interface FlowVisitor {
  hookRequestheaders?: Handler;
  hookRequest?: Handler;
  hookResponseheaders?: Handler;
  hookResponse?: Handler;
}

/** @file implements the websocket protocol used by the distillery */

import WebSocket from 'isomorphic-ws'
import { Buffer } from 'buffer'

const EXIT_STATUS_NORMAL_CLOSE = 1000;
const PROTOCOL = 'pow-1';

/** Call represents a specific WebSocket call */
export default class Call {
  constructor (remote: Remote, spec: CallSpec) {
    this.remote = remote
    this.call = spec
  }

  public readonly remote: Readonly<Remote>
  public readonly call: Readonly<CallSpec>

  /** called right before sending the request */
  public beforeCall?: (this: Call) => void

  /** called right after the connection has been established */
  public onConnect?: (this: Call) => void

  /** called right after the socket is closed */
  public afterCall?: (this: Call, result: Result) => void

  /** called when an error occurs before rejecting the promise */
  public onError?: (this: Call, error: any) => void

  /** called when a log line is received */
  public onLogLine?: (this: Call, line: string) => void

  /** connect checks if the connect method was called */
  private connected: boolean = false

  /** holds the websocket when the connection is alive */
  private ws: WebSocket | null = null

  /**
   * Connect to the specified remote endpoint and perform the action
   * @param remote Remote to connect to
   */
  async connect (): Promise<Result> {
    // ensure that connect is only run once.
    if (this.connected) {
      throw new Error('connect() may only be called once')
    }
    this.connected = true

    // and do the connection!
    return new Promise((resolve, reject) => {
      // create the websocket
      const ws = new WebSocket(this.remote.url, PROTOCOL, typeof this.remote.token === 'string' ? { headers: { Authorization: 'Bearer ' + this.remote.token } } : undefined)
      this.ws = ws // make it available to other thing

      ws.onopen = () => {
        this._closeStateHack();

        if (this.beforeCall != null) {
          this.beforeCall()
        }

        ws.send(Buffer.from(JSON.stringify(this.call), 'utf8'))

        if (this.onConnect != null) {
          this.onConnect()
        }
      }

      ws.onmessage = ({ data, ...rest }: { data: unknown }) => {
        // ignore non-strings for now
        if (typeof data !== 'string') {
          // TODO: protocol error
          return
        }

        if (this.onLogLine != null) {
          this.onLogLine(data)
        }
        return
      }

      ws.onerror = (err: unknown) => {
        this.close()

        // call the handler and reject
        if (this.onError != null) {
          this.onError(err)
        }
        reject(err)
      }

      ws.onclose = (event: { code: number; reason: string; wasClean: boolean}) => {
        // normal close => process succeeded
        if (event.code !== EXIT_STATUS_NORMAL_CLOSE) {
          resolve({ success: false, data: event.reason });
          return;
        } 
        
        let reason: unknown
        try {
          reason = JSON.parse(event.reason)
        } catch (e: unknown) {
          resolve({ success: false, data: "protocol error: unable to parse reason field"})
          return; 
        }

        if (typeof reason !== 'object' || !reason) {
          resolve({ success: false, data: "protocol error: reason field is not an object"});
          return;
        }

        const { success, data } = reason as any;
        
        if (typeof success !== 'boolean') {
          resolve({ success: false, data: "protocol error: success field not a boolean"});
          return;
        }

        if (success === false) {
          if (typeof data !== 'string') {
            resolve({ success: false, data: "protocol error: data field does not contain a message"});
            return;
          }

          resolve({ success: false, data: data })
          return;
        } 
        resolve({ success: true, data: data });
        
        this.close();
      };
    });
  }

  /**
   * Sometimes for unknown reasons the websocket gets stuck in CLOSING state.
   * 
   * This code triggers code to manually unstick the server
   */
  private _closeStateHack() {
    const STATE_POLL_INTERVAL = 100;  // how often to poll the state
    const CLOSE_TIMEOUT = 500;        // how long to wait for the close to finish on it's own 

    const poller = setInterval(() => {
      // if we have an open or connecting websocket keep going
      const ws = this.ws;
      if (ws !== null && (ws.readyState === ws.OPEN || ws.readyState === ws.CONNECTING)) {
        return;
      }

      // clear the interval and only continue if in CLOSING state
      clearInterval(poller);
      if (ws === null || ws.readyState !== ws.CLOSING) {
        return
      }

      setTimeout(() => {
        if (ws.readyState === ws.CLOSING) {
          console.warn('websocket client misbehaved: still in closing state')
          ws.terminate();
        }
      }, CLOSE_TIMEOUT);
    }, STATE_POLL_INTERVAL);
  }

  /** sendText sends some text to the server requests cancellation of an ongoing operation */
  sendText (text: string): void {
    const ws = this.ws
    if (ws == null) {
      throw new Error('websocket not connected')
    }

    ws.send(text)
  }

  /** cancel requests cancellation of an ongoing operation */
  cancel (): void {
    const ws = this.ws
    if (ws == null) {
      throw new Error('websocket not connected')
    }

    ws.send(Buffer.from(JSON.stringify({ signal: 'cancel' }), 'utf8'))
  }

  /** 
   * closeInput closes the input from the client
   * Any further text received on the server side will be ignored.
   */
  closeInput (): void {
    const ws = this.ws
    if (ws == null) {
      throw new Error('websocket not connected')
    }

    ws.send(Buffer.from(JSON.stringify({ signal: 'close' }), 'utf8'))
  }

  /** close closes this websocket connection */
  private close (): void {
    const ws = this.ws
    if (ws == null) {
      throw new Error('websocket not connected')
    }

    ws.close()
    this.ws = null
  }
}

/** specifies a remote endpoint */
export interface Remote {
  url: string // the remote websocket url to talk to
  token?: string // optional token
}

/** CallSpec represents the specification for a call */
export interface CallSpec {
  call: string
  params: string[]
}

/** the result of a websocket call */
export type Result = ResultSuccess | ResultFailure
interface ResultSuccess {
  success: true
  data: unknown
}

interface ResultFailure {
  success: false
  data: string // error message (if any)
}

# Process Over Websocket

Process Over Websocket is a protocol and implementation for running arbitrary, server-defined processes via a websocket-based and also rest-based API. 
It is intended for use by the [WissKI Distillery](https://github.com/FAU-CDI/wisski-distillery).  

This repository contains a server written in [Go](https://go.dev/), as well as a client written in [Typescript](https://www.typescriptlang.org/). 
An example server can be found in [in the exampleserver directory](cmd/exampleserver).
An example client can be found in [the client directory](client/examples). 

## Protocol Overview

A process is an arbitrary action performed by a server. 
A process is created by passing the name of the process and parameters to a server. 

The server then creates associated input and output streams. 
The client can send data to the input stream, or even close it. 
It can also ask the server to cancel an ongoing process. 

Once a process finishes (be it by naturally being completed, or by a user requesting cancellation), it will either return an error message or a result. 

## Websocket API

Clients can connect to the websocket server using an path. 
Client must connect using the `pow-1` subprotocol.

After the handshake is completed, servers and clients exchange two kinds of frames:

- binary frames, which are json-encoded and used for control flow.
- text frames, which are used to send input and output.

Text frames may be sent at any point by either side once the process has started. 
Frames send from the client to the server are forwarded to the ongoing process; frames sent from the server to the client are output produced by the process. 

The following binary frame types exist:

**Call Message**. 

This message must be sent from the client to the server to start a process. 
It should contain two fields. 
The `call` field containing the name of the process as a string. 
The `params` field should be an array of parameters to pass to the process. The params should be strings. 

**CloseInput Message**.

This message may be sent from the client to the server to close the process's standard input. 
Any future input will be ignored. 

It should be the json object `{"signal":"close"}`.

**Cancel Message**.

This message may be sent from the client to the server to request the process to be cancelled.  

It should be the json object `{"signal":"cancel"}`.

**Close Frame & Result Message**

When the process finishes, the server sends a json-encoded binary frame to the client.
- success messages contain the field `status` set to the string `"fulfilled"` and the optional `value` field with a json-encoded process result.
- failure messaged contain the field `status` set to the string `"rejected"` and the optional `reason` field with a string containing an error message.

Afterwards the server sends a close frame with the normal closure code and an empty reason field. 

## REST API

The REST API is documented using an OpenAPI specification. 
See [openapi.json](internal/rest_impl/openapi.json) for details.
By default, the server also serves a [SwaggerUI](https://swagger.io/tools/swagger-ui/) at `/docs/`.

## LICENSE

This code and associated documentation are licensed under AGPL-3.0. 
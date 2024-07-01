import { WebsocketSession } from '../pow_client'

const session = new WebsocketSession({ url: 'ws://localhost:3000' }, { call: 'echo', params: ['random', 'params'] })
session.onLogLine = console.log

void (
  async (): Promise<void> => {
    await session.connect()

    await session.sendText('hello')
    await session.sendText('world')
    await session.closeInput()

    const result = await session.wait()
    console.log(result)
  }
)().catch((err: any) => { console.error(err.message) })

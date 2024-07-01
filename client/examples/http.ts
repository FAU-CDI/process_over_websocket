import { RestSession } from '../pow_client'

const session = new RestSession({ url: 'http://localhost:3000' }, { call: 'echo', params: ['random', 'params'] })

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

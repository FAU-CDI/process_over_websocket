import Call from '../../websocket';


const call = new Call({ url: "ws://localhost:3000" }, { call: 'echo', params: ['random', 'params']});
call.onLogLine = console.log
call.onConnect = function() {
    this.sendText('hello')
    this.sendText('world')
    this.closeInput()
}
call.connect().then(console.info).catch(console.error)
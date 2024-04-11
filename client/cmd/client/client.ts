import Call from '../../websocket';


const call = new Call({ url: "ws://localhost:3000" }, { call: 'tick', params: []});
call.onLogLine = console.log
call.onConnect = function() {
    setTimeout(() => this.cancel(), 10 * 1000);
}
call.connect().then(console.info).catch(console.error)
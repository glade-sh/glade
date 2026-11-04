// VitePress preview listens on an unspecified address, which can coexist with
// a separate IPv4 listener on macOS. Bind the address checked by the smoke test.
const net = require('node:net')
const listen = net.Server.prototype.listen
net.Server.prototype.listen = function (...args) {
  if (args[0] === 4173 && (args.length === 1 || typeof args[1] === 'function')) {
    args.splice(1, 0, '127.0.0.1')
  }
  return listen.apply(this, args)
}

// The runner has no route out: a TCP connection must fail, and fail fast.
const net = require("net");

function tcp(host, port) {
  return new Promise((resolve) => {
    const socket = net.connect({ host, port, timeout: 2000 });
    socket.on("connect", () => {
      socket.destroy();
      resolve("open");
    });
    socket.on("timeout", () => {
      socket.destroy();
      resolve("blocked");
    });
    socket.on("error", () => resolve("blocked"));
  });
}

(async () => {
  console.log(`tcp: ${await tcp("1.1.1.1", 53)}`);
  console.log(`tcp: ${await tcp("8.8.8.8", 443)}`);
})();

import { d as defineEventHandler } from '../../../nitro/nitro.mjs';
import { o as onStoreUpdates } from '../../../_/alert-store.mjs';
import 'node:http';
import 'node:https';
import 'node:events';
import 'node:buffer';
import 'node:fs';
import 'node:path';
import 'node:crypto';
import 'node:url';

function sendSSE(res, eventName, data) {
  res.write(`event: ${eventName}
`);
  res.write(`data: ${JSON.stringify(data)}

`);
}
const stream_get = defineEventHandler((event) => {
  var _a;
  const res = event.node.res;
  res.setHeader("Content-Type", "text/event-stream");
  res.setHeader("Cache-Control", "no-cache, no-transform");
  res.setHeader("Connection", "keep-alive");
  (_a = res.flushHeaders) == null ? void 0 : _a.call(res);
  sendSSE(res, "ready", { ok: true });
  const unsubscribe = onStoreUpdates((alerts) => {
    sendSSE(res, "alerts", { alerts });
  });
  const keepAlive = setInterval(() => {
    sendSSE(res, "heartbeat", { at: (/* @__PURE__ */ new Date()).toISOString() });
  }, 15e3);
  event.node.req.on("close", () => {
    clearInterval(keepAlive);
    unsubscribe();
    res.end();
  });
});

export { stream_get as default };
//# sourceMappingURL=stream.get.mjs.map

import { d as defineEventHandler, g as getRouterParam, c as createError } from '../../../nitro/nitro.mjs';
import { g as getAlert } from '../../../_/alert-store.mjs';
import 'node:http';
import 'node:https';
import 'node:events';
import 'node:buffer';
import 'node:fs';
import 'node:path';
import 'node:crypto';
import 'node:url';

const _id__get = defineEventHandler((event) => {
  const id = getRouterParam(event, "id");
  const alert = id ? getAlert(id) : void 0;
  if (!alert) {
    throw createError({ statusCode: 404, statusMessage: "Alert not found" });
  }
  return { alert };
});

export { _id__get as default };
//# sourceMappingURL=_id_.get.mjs.map

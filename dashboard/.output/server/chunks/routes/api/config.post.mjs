import { d as defineEventHandler, r as readBody } from '../../nitro/nitro.mjs';
import { e as setWorkflowConfig } from '../../_/alert-store.mjs';
import 'node:http';
import 'node:https';
import 'node:events';
import 'node:buffer';
import 'node:fs';
import 'node:path';
import 'node:crypto';
import 'node:url';

const config_post = defineEventHandler(async (event) => {
  const payload = await readBody(event);
  const config = setWorkflowConfig(payload);
  return { ok: true, config };
});

export { config_post as default };
//# sourceMappingURL=config.post.mjs.map

import { d as defineEventHandler } from '../../nitro/nitro.mjs';
import { c as getWorkflowConfig } from '../../_/alert-store.mjs';
import 'node:http';
import 'node:https';
import 'node:events';
import 'node:buffer';
import 'node:fs';
import 'node:path';
import 'node:crypto';
import 'node:url';

const config_get = defineEventHandler(() => ({ config: getWorkflowConfig() }));

export { config_get as default };
//# sourceMappingURL=config.get.mjs.map

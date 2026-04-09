/**
 * Restricts which hosts the dashboard may proxy to for MCP tools (SSRF mitigation).
 * Override with comma-separated hostnames, e.g. TOOLS_ENDPOINT_ALLOWED_HOSTS=localhost,127.0.0.1,mcp.internal
 */
const defaultAllowed = new Set([
  'localhost',
  '127.0.0.1',
  '::1',
  'host.docker.internal'
])

function allowedHosts(): Set<string> {
  const env = process.env.TOOLS_ENDPOINT_ALLOWED_HOSTS
  if (!env?.trim()) {
    return defaultAllowed
  }
  const s = new Set<string>()
  for (const h of env.split(',')) {
    const t = h.trim().toLowerCase()
    if (t) s.add(t)
  }
  return s.size > 0 ? s : defaultAllowed
}

/**
 * Returns normalized base URL string or throws createError-compatible message.
 */
export function assertAllowedToolsBaseUrl(raw: string): string {
  const baseUrl = raw.trim().replace(/\/$/, '')
  if (!/^https?:\/\//i.test(baseUrl)) {
    throw new Error('Tools endpoint must start with http:// or https://')
  }
  let parsed: URL
  try {
    parsed = new URL(baseUrl)
  } catch {
    throw new Error('Invalid tools endpoint URL')
  }
  const host = parsed.hostname.toLowerCase()
  const allow = allowedHosts()
  if (!allow.has(host)) {
    throw new Error(
      `Tools endpoint host "${host}" is not allowed. Set TOOLS_ENDPOINT_ALLOWED_HOSTS to add hosts (comma-separated).`
    )
  }
  return baseUrl
}

export function formatWhen(value: unknown) {
  if (!value) return 'Just now'
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function htmlToMarkdown(input: string): string {
  console.log('htmlToMarkdown input:', input)
  // First decode HTML entities
  let text = decodeHtmlEntities(input)
  console.log('after decodeHtmlEntities:', text)
  
  // Step 1: Handle <br> tags - convert to newlines
  // Convert <br> or <br/> to newline, handle multiple <br> as paragraph breaks
  text = text.replace(/<br\s*\/?>\s*<br\s*\/?>/gi, '\n\n')
  text = text.replace(/<br\s*\/?>/gi, '\n')
  console.log('after <br> replacement:', text)
  
  // Step 2: Fix common malformed patterns
  // Fix duplicate code block markers (e.g., ```bash\n```bash\n)
  // Pattern: ```lang\n```lang\ncontent\n```
  text = text.replace(/```(\w+)\s*\n\s*```\1\s*\n/g, '```$1\n')
  // Pattern: ```\n```\ncontent\n```
  text = text.replace(/```\s*\n\s*```\s*\n/g, '```\n')
  // Fix: ```bash\n```bash (without newline between them)
  text = text.replace(/```(\w+)```\1/g, '```$1')
  // Fix: ```\n``` (consecutive without content)
  text = text.replace(/```\s*```/g, '```')
  console.log('after fixing malformed patterns:', text)
  
  // Step 3: Convert HTML tags to markdown
  // Process each tag type
  text = text
    .replace(/<strong>([\s\S]*?)<\/strong>/gi, '**$1**')
    .replace(/<b>([\s\S]*?)<\/b>/gi, '**$1**')
    .replace(/<em>([\s\S]*?)<\/em>/gi, '*$1*')
    .replace(/<i>([\s\S]*?)<\/i>/gi, '*$1*')
    .replace(/<code>([\s\S]*?)<\/code>/gi, '`$1`')
    .replace(/<pre>([\s\S]*?)<\/pre>/gi, '```\n$1\n```')
    .replace(/<h1>([\s\S]*?)<\/h1>/gi, '# $1')
    .replace(/<h2>([\s\S]*?)<\/h2>/gi, '## $1')
    .replace(/<h3>([\s\S]*?)<\/h3>/gi, '### $1')
    .replace(/<ul>([\s\S]*?)<\/ul>/gi, '$1')
    .replace(/<ol>([\s\S]*?)<\/ol>/gi, '$1')
    .replace(/<li>([\s\S]*?)<\/li>/gi, '- $1')
    .replace(/<p>([\s\S]*?)<\/p>/gi, '$1\n\n')
    .replace(/<a href="([\s\S]*?)">([\s\S]*?)<\/a>/gi, '[$2]($1)')
  console.log('after HTML tag conversion:', text)
  
  // Step 4: Remove any remaining HTML tags
  text = text.replace(/<[^>]*>/g, '')
  console.log('after removing remaining HTML tags:', text)
  
  // Step 5: Clean up markdown formatting issues
  // Fix code blocks with language but no content
  text = text.replace(/```(\w+)\s*\n\s*```/g, '```$1\n```')
  
  // Normalize newlines (3+ newlines -> 2 newlines)
  text = text.replace(/\n{3,}/g, '\n\n')
  
  const result = text.trim()
  console.log('htmlToMarkdown result:', result)
  return result
}

export function decodeHtmlEntities(input: string): string {
  if (typeof window === 'undefined') {
    // Fallback for SSR: replace common entities
    return input
      .replace(/&lt;/g, '<')
      .replace(/&gt;/g, '>')
      .replace(/&amp;/g, '&')
      .replace(/&quot;/g, '"')
      .replace(/&#39;/g, "'")
      .replace(/&nbsp;/g, ' ')
  }
  // Use browser's DOMParser for accurate decoding
  const doc = new DOMParser().parseFromString(input, 'text/html')
  return doc.documentElement.textContent || input
}

export function tryFormatJson(input: string): string {
  const trimmed = input.trim()
  // Check if it looks like JSON (starts with { or [ and ends with } or ])
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || 
      (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      const parsed = JSON.parse(trimmed)
      // Pretty print with 2-space indentation
      const formatted = JSON.stringify(parsed, null, 2)
      // Wrap in json code block for syntax highlighting
      return `\`\`\`json\n${formatted}\n\`\`\``
    } catch (e) {
      // Not valid JSON, return original
      return input
    }
  }
  return input
}

export function simpleMarkdownToHtml(text: string): string {
  // Very basic markdown to HTML conversion for fallback
  let result = ''
  let pos = 0
  const codeBlockRegex = /```(\w*)\n([\s\S]*?)\n```/g
  let match
  
  // Process code blocks first
  while ((match = codeBlockRegex.exec(text)) !== null) {
    // Text before the code block
    const before = text.slice(pos, match.index)
    // Escape HTML in regular text
    result += before.replace(/</g, '&lt;').replace(/>/g, '&gt;')
    
    const lang = match[1]
    const code = match[2]
    const languageClass = lang ? ` class="language-${lang}"` : ''
    const escapedCode = code.replace(/</g, '&lt;').replace(/>/g, '&gt;')
    result += `<pre><code${languageClass}>${escapedCode}</code></pre>`
    pos = match.index + match[0].length
  }
  
  // Remaining text after last code block
  const remaining = text.slice(pos)
  result += remaining.replace(/</g, '&lt;').replace(/>/g, '&gt;')
  
  // Now convert inline markdown in the escaped text (but not inside <pre> tags)
  // Simple approach: convert inline code, bold, italic
  // Since we've already escaped HTML, we can safely apply regex
  result = result.replace(/`([^`]+)`/g, '<code>$1</code>')
  result = result.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  result = result.replace(/__([^_]+)__/g, '<strong>$1</strong>')
  result = result.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  result = result.replace(/_([^_]+)_/g, '<em>$1</em>')
  
  return result
}

export function renderMarkdown(text: string): string {
  console.log('renderMarkdown input:', text)
  // Type augmentation for marked and hljs loaded via CDN
  const win = window as typeof window & { marked?: any; hljs?: any }
  // Decode HTML entities first (e.g., &lt;br&gt; -> <br>)
  const decodedText = decodeHtmlEntities(text)
  console.log('decodedText:', decodedText)
  
  // Convert HTML tags to markdown equivalents
  const processedText = htmlToMarkdown(decodedText)
  console.log('processedText:', processedText)
  
  // Try to format JSON if the whole text is a JSON object/array
  const formattedText = tryFormatJson(processedText)
  console.log('formattedText:', formattedText)
  
  if (typeof window === 'undefined' || !win.marked) {
    console.warn('marked not available, using simple markdown fallback')
    return simpleMarkdownToHtml(formattedText)
  }
  const marked = win.marked
  const hljs = win.hljs
  console.log('marked available:', !!marked, 'hljs available:', !!hljs)
  marked.setOptions({
    breaks: true,
    gfm: true,
    headerIds: false,
    highlight: (code: string, lang?: string) => {
      if (hljs && hljs.getLanguage) {
        const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext'
        try {
          return hljs.highlight(code, { language }).value
        } catch (e) {
          console.warn('hljs highlight failed:', e)
        }
      }
      // Return false to let marked use default escaping
      return false
    }
  })
  const result = marked.parse(formattedText)
  console.log('marked parse result:', result)
  return result
}
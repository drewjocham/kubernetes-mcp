// Simulate the frontend rendering pipeline
function decodeHtmlEntities(input) {
  // Simple replacement (same as frontend SSR fallback)
  return input
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, ' ')
}

function htmlToMarkdown(input) {
  let text = decodeHtmlEntities(input)
  
  // Step 1: Handle <br> tags - convert to newlines
  text = text.replace(/<br\s*\/?>\s*<br\s*\/?>/gi, '\n\n')
  text = text.replace(/<br\s*\/?>/gi, '\n')
  
  // Step 2: Fix common malformed patterns
  text = text.replace(/```(\w+)\s*\n\s*```\1\s*\n/g, '```$1\n')
  text = text.replace(/```\s*\n\s*```\s*\n/g, '```\n')
  text = text.replace(/```(\w+)```\1/g, '```$1')
  text = text.replace(/```\s*```/g, '```')
  
  // Step 3: Convert HTML tags to markdown
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
  
  // Step 4: Remove any remaining HTML tags
  text = text.replace(/<[^>]*>/g, '')
  
  // Step 5: Clean up markdown formatting issues
  text = text.replace(/```(\w+)\s*\n\s*```/g, '```$1\n```')
  text = text.replace(/\n{3,}/g, '\n\n')
  
  return text.trim()
}

function tryFormatJson(input) {
  const trimmed = input.trim()
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || 
      (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      const parsed = JSON.parse(trimmed)
      const formatted = JSON.stringify(parsed, null, 2)
      return `\`\`\`json\n${formatted}\n\`\`\``
    } catch (e) {
      return input
    }
  }
  return input
}

function simpleMarkdownToHtml(text) {
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
  result = result.replace(/`([^`]+)`/g, '<code>$1</code>')
  result = result.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  result = result.replace(/__([^_]+)__/g, '<strong>$1</strong>')
  result = result.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  result = result.replace(/_([^_]+)_/g, '<em>$1</em>')
  
  // Convert line breaks
  result = result.replace(/\n/g, '<br>')
  
  return result
}

// Test the error example
const errorExample = 'Arguskube encountered an error: ```json<br>{<br> &nbsp;"type": "error",<br> &nbsp;"error": {<br> &nbsp; &nbsp;"type": "FreeUsageLimitError",<br> &nbsp; &nbsp;"message": "Rate limit exceeded. Please try again later."<br> &nbsp;}<br>}<br>```';

console.log('=== INPUT ===');
console.log(errorExample);

console.log('\n=== decodeHtmlEntities ===');
const decoded = decodeHtmlEntities(errorExample);
console.log(decoded);

console.log('\n=== htmlToMarkdown ===');
const processed = htmlToMarkdown(decoded);
console.log(processed);

console.log('\n=== tryFormatJson (should not change) ===');
const formatted = tryFormatJson(processed);
console.log(formatted);

console.log('\n=== simpleMarkdownToHtml ===');
const html = simpleMarkdownToHtml(formatted);
console.log(html);

console.log('\n=== HTML preview (copy buttons target <pre>) ===');
const preCount = (html.match(/<pre>/g) || []).length;
console.log(`Found ${preCount} <pre> elements`);
if (preCount > 0) {
  console.log('Copy buttons should work!');
} else {
  console.log('ERROR: No <pre> elements generated');
}
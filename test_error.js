function decodeHtmlEntities(input) {
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

function htmlToMarkdown(input) {
  // First decode HTML entities
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

// Simulate browser environment
global.window = { DOMParser: class DOMParser {
  parseFromString(str, type) {
    // Simulate DOMParser behavior for HTML entities
    const parser = require('htmlparser2');
    const handler = new parser.DomHandler((error, dom) => {
      if (error) throw error;
    });
    const parse = new parser.Parser(handler);
    parse.write(str);
    parse.end();
    // Simple decoding for test
    let decoded = str
      .replace(/&lt;/g, '<')
      .replace(/&gt;/g, '>')
      .replace(/&amp;/g, '&')
      .replace(/&quot;/g, '"')
      .replace(/&#39;/g, "'")
      .replace(/&nbsp;/g, ' ');
    return {
      documentElement: {
        textContent: decoded
      }
    };
  }
} };

const errorExample = 'Arguskube encountered an error: ```json<br>{<br> &nbsp;"type": "error",<br> &nbsp;"error": {<br> &nbsp; &nbsp;"type": "FreeUsageLimitError",<br> &nbsp; &nbsp;"message": "Rate limit exceeded. Please try again later."<br> &nbsp;}<br>}<br>```';

console.log('Input:');
console.log(errorExample);
console.log('\n---\nAfter decodeHtmlEntities:');
const decoded = decodeHtmlEntities(errorExample);
console.log(decoded);
console.log('\n---\nAfter htmlToMarkdown:');
const result = htmlToMarkdown(errorExample);
console.log(result);

function decodeHtmlEntities(input) {
  let result = input;
  result = result.replace(/&lt;/g, '<');
  result = result.replace(/&gt;/g, '>');
  result = result.replace(/&amp;/g, '&');
  result = result.replace(/&quot;/g, '"');
  result = result.replace(/&#39;/g, "'");
  result = result.replace(/&nbsp;/g, ' ');
  return result;
}

function cleanupMarkdown(input) {
  if (!input) return input;
  
  // Decode HTML entities first
  let result = decodeHtmlEntities(input);
  
  // Replace HTML line breaks with newlines
  result = result.replace(/<br>/g, '\n');
  result = result.replace(/<br\/>/g, '\n');
  result = result.replace(/<br \/>/g, '\n');
  
  // Fix duplicate consecutive code block markers
  const lines = result.split('\n');
  const cleanedLines = [];
  let inCodeBlock = false;
  let prevLine = '';
  
  for (const line of lines) {
    const trimmed = line.trim();
    
    if (trimmed.startsWith('```')) {
      if (inCodeBlock) {
        // Already in code block, might be duplicate opener
        // Skip if previous line was also a code block opener
        if (prevLine.trim().startsWith('```')) {
          continue; // Skip duplicate opener
        }
      }
      inCodeBlock = !inCodeBlock;
    }
    
    cleanedLines.push(line);
    prevLine = line;
  }
  
  result = cleanedLines.join('\n');
  
  // Remove any remaining HTML tags (simple approach)
  result = result.replace(/<strong>/g, '**');
  result = result.replace(/<\/strong>/g, '**');
  result = result.replace(/<b>/g, '**');
  result = result.replace(/<\/b>/g, '**');
  result = result.replace(/<em>/g, '*');
  result = result.replace(/<\/em>/g, '*');
  result = result.replace(/<i>/g, '*');
  result = result.replace(/<\/i>/g, '*');
  
  // Remove any other HTML tags (crude but works for common cases)
  while (result.includes('<') && result.includes('>')) {
    const start = result.indexOf('<');
    const end = result.indexOf('>');
    if (start >= 0 && end > start) {
      result = result.slice(0, start) + result.slice(end + 1);
    } else {
      break;
    }
  }
  
  // Normalize newlines (3+ newlines -> 2 newlines)
  while (result.includes('\n\n\n')) {
    result = result.replace(/\n\n\n/g, '\n\n');
  }
  
  return result.trim();
}

// Test the exact error example
const errorExample = 'Arguskube encountered an error: ```json<br>{<br> &nbsp;"type": "error",<br> &nbsp;"error": {<br> &nbsp; &nbsp;"type": "FreeUsageLimitError",<br> &nbsp; &nbsp;"message": "Rate limit exceeded. Please try again later."<br> &nbsp;}<br>}<br>```';

console.log('Input:');
console.log(errorExample);
console.log('\n---\nAfter cleanupMarkdown:');
const cleaned = cleanupMarkdown(errorExample);
console.log(cleaned);
console.log('\n---\nExpected markdown for rendering:');
console.log('Should have proper code blocks with newlines, not <br> tags.');

// Also test frontend functions (simplified)
function decodeHtmlEntitiesFrontend(input) {
  // Use simple replacement for test
  return decodeHtmlEntities(input);
}

function htmlToMarkdown(input) {
  let text = decodeHtmlEntitiesFrontend(input);
  text = text.replace(/<br\s*\/?>\s*<br\s*\/?>/gi, '\n\n');
  text = text.replace(/<br\s*\/?>/gi, '\n');
  // Additional conversions...
  text = text.replace(/<[^>]*>/g, '');
  return text.trim();
}

console.log('\n---\nFrontend simulation:');
console.log('Decoded:', decodeHtmlEntitiesFrontend(errorExample));
console.log('htmlToMarkdown:', htmlToMarkdown(errorExample));
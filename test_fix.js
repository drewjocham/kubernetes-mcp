function decodeHtmlEntities(input) {
  // Fallback for SSR: replace common entities
  return input
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, ' ')
}

function htmlToMarkdown(input) {
  // First decode HTML entities
  let text = decodeHtmlEntities(input)
  
  console.log('Step 1 - after decode:', text)
  
  // Handle <br> tags - convert to newlines, collapse multiple
  text = text.replace(/<br\s*\/?>\s*<br\s*\/?>/gi, '\n\n')
  text = text.replace(/<br\s*\/?>/gi, '\n')
  console.log('Step 2 - after br handling:', text)
  
  // Handle code blocks - need to be careful with nested pre/code
  // First extract pre blocks to protect them
  const preBlocks = []
  let preIndex = 0
  text = text.replace(/<pre>([\s\S]*?)<\/pre>/gi, (match, content) => {
    preBlocks.push(content)
    return `__PRE_BLOCK_${preIndex++}__`
  })
  console.log('Step 3 - after pre extraction:', text)
  console.log('Pre blocks:', preBlocks)
  
  // Now process other tags
  text = text
    .replace(/<strong>([\s\S]*?)<\/strong>/gi, '**$1**')
    .replace(/<b>([\s\S]*?)<\/b>/gi, '**$1**')
    .replace(/<em>([\s\S]*?)<\/em>/gi, '*$1*')
    .replace(/<i>([\s\S]*?)<\/i>/gi, '*$1*')
    .replace(/<code>([\s\S]*?)<\/code>/gi, '`$1`')
    .replace(/<h1>([\s\S]*?)<\/h1>/gi, '# $1')
    .replace(/<h2>([\s\S]*?)<\/h2>/gi, '## $1')
    .replace(/<h3>([\s\S]*?)<\/h3>/gi, '### $1')
    .replace(/<ul>([\s\S]*?)<\/ul>/gi, '$1')
    .replace(/<ol>([\s\S]*?)<\/ol>/gi, '$1')
    .replace(/<li>([\s\S]*?)<\/li>/gi, '- $1')
    .replace(/<p>([\s\S]*?)<\/p>/gi, '$1\n\n')
    .replace(/<a href="([\s\S]*?)">([\s\S]*?)<\/a>/gi, '[$2]($1)')
  
  console.log('Step 4 - after tag processing:', text)
  
  // Restore pre blocks
  text = text.replace(/__PRE_BLOCK_(\d+)__/g, (match, index) => {
    return `\`\`\`\n${preBlocks[index]}\n\`\`\``
  })
  console.log('Step 5 - after pre restoration:', text)
  
  // Clean up - remove any remaining HTML tags
  text = text.replace(/<[^>]*>/g, '')
  
  // Clean up extra newlines
  text = text.replace(/\n{3,}/g, '\n\n')
  
  return text.trim()
}

// Test with the problematic example
const example = `# Minikube<br><br>Minikube is a tool that lets you run a **single-node Kubernetes cluster** locally on your machine. It's great for learning Kubernetes, testing locally, or developing applications.<br><br>## Quick Start<br><br><br>\`\`\`bash<br>\`\`\`bash<br># Start a cluster<br>minikube start<br>\`\`\`<br><br><br># Check status<br>minikube status<br><br># Stop the cluster<br>minikube stop<br><br># Delete the cluster<br>minikube delete<br>\`\`\`<br><br>## Common Commands<br><br><br>\`\`\`bash<br>\`\`\`bash<br># Open Kubernetes dashboard<br>minikube dashboard<br>\`\`\`<br><br><br># Get the IP address of the cluster<br>minikube ip<br><br># SSH into the minikube node<br>minikube ssh<br><br># Enable addons (e.g., ingress, storageclass)<br>minikube addons enable ingress<br>minikube addons list<br>\`\`\`<br><br>## How can I help you?<br><br>- 🚀 **Setup** - Install and configure minikube<br>- 🛠️ **Troubleshooting** - Fix errors or issues<br>- 📦 **Deployment** - Deploy applications to minikube<br>- 🔧 **Configuration** - Customize minikube settings (VM driver, resources, etc.)<br>- 🔌 **Addons** - Enable ingress, metrics-server, etc.<br><br>What would you like to do with minikube?`

console.log('Input:')
console.log(example)
console.log('\n---\nOutput:')
const result = htmlToMarkdown(example)
console.log(result)

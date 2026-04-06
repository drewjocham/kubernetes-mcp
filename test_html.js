function htmlToMarkdown(input) {
  return input
    .replace(/<br\s*\/?>/gi, '\n\n')
    .replace(/<strong>([\s\S]*?)<\/strong>/gi, '**$1**')
    .replace(/<b>([\s\S]*?)<\/b>/gi, '**$1**')
    .replace(/<em>([\s\S]*?)<\/em>/gi, '*$1*')
    .replace(/<i>([\s\S]*?)<\/i>/gi, '*$1*')
    .replace(/<code>([\s\S]*?)<\/code>/gi, '\`$1\`')
    .replace(/<pre>([\s\S]*?)<\/pre>/gi, '```\n$1\n```')
    .replace(/<h1>([\s\S]*?)<\/h1>/gi, '# $1')
    .replace(/<h2>([\s\S]*?)<\/h2>/gi, '## $1')
    .replace(/<h3>([\s\S]*?)<\/h3>/gi, '### $1')
    .replace(/<ul>([\s\S]*?)<\/ul>/gi, '$1')
    .replace(/<ol>([\s\S]*?)<\/ol>/gi, '$1')
    .replace(/<li>([\s\S]*?)<\/li>/gi, '- $1')
    .replace(/<p>([\s\S]*?)<\/p>/gi, '$1\n\n')
    .replace(/<a href="([\s\S]*?)">([\s\S]*?)<\/a>/gi, '[$2]($1)');
}

const example = `I'd be happy to help you check on your cluster! However, I need a bit more context:<br><br>**What type of cluster are you asking about?**<br><br>- **Kubernetes cluster** (e.g., \`kubectl cluster-info\`, \`kubectl get nodes\`)<br>- **HPC/Compute cluster** (e.g., SLURM, PBS)<br>- **Database cluster** (e.g., PostgreSQL, Redis, MySQL)<br>- **Cloud resource cluster** (e.g., AWS, GCP, Azure)<br>- **Something else?**<br><br>Also, if you can share:<br><br>- Any error messages or logs you're seeing<br>- The command output you've tried already<br>- What specifically seems "off" (e.g., nodes down, slow performance, connection issues)<br><br>I can then help you diagnose the issue!`;

console.log('Input:');
console.log(example);
console.log('\n---\nOutput:');
console.log(htmlToMarkdown(example));

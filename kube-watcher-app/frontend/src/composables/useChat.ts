import { ref, computed } from 'vue'
import { AskAI } from '../../wailsjs/go/main/App'
import { data } from '../../wailsjs/go/models'

type ChatMessage = {
  role: 'assistant' | 'user'
  content: string
  tone?: string
}

export function useChat() {
  const draftPrompt = ref('')
  const chatMessages = ref<ChatMessage[]>([
    {
      role: 'assistant',
      tone: 'calm',
      content: 'Arguskube Sentinels are ready to investigate the cluster. Load the workspace, then ask for rollout prep, incident triage, or anomaly validation.',
    },
  ])
  const isThinking = ref(false)

  const sendPrompt = async (prompt: string, context?: string) => {
    const trimmed = prompt.trim()
    if (!trimmed) return

    chatMessages.value.push({ role: 'user', content: trimmed })
    draftPrompt.value = ''
    isThinking.value = true

    try {
      const response = await AskAI(trimmed, context || '')
      const formattedResponse = formatAIResponse(response)
      chatMessages.value.push({ role: 'assistant', tone: 'calm', content: formattedResponse })
    } catch (error) {
      chatMessages.value.push({
        role: 'assistant',
        tone: 'warn',
        content: `Arguskube encountered an error: ${formatError(error)}`,
      })
    } finally {
      isThinking.value = false
    }
  }

  const formatError = (error: unknown): string => {
    let msg = error instanceof Error ? error.message : String(error);
    // Detect chat completion errors with raw JSON
    const match = msg.match(/chat completion error \(\d+\):\s*(.*)$/i);
    if (match) {
      try {
        const jsonStr = match[1].trim();
        const parsed = JSON.parse(jsonStr);
        const pretty = JSON.stringify(parsed, null, 2);
        return `\`\`\`json\n${pretty}\n\`\`\``;
      } catch (parseErr) {
        console.warn('Failed to format error JSON:', parseErr);
      }
    }
    return msg;
  }

  const formatAIResponse = (response: string): string => {
    if (!response) return response
    
    // Split into lines and process
    const lines = response.split('\n')
    let formatted = ''
    let inCodeBlock = false
    let codeBlockContent = ''
    
    for (let i = 0; i < lines.length; i++) {
      const originalLine = lines[i]
      const line = originalLine.trim()
      
      // Detect if this line looks like a command
      const isCommand = line.match(/^(kubectl|kw|docker|helm|git|ls|cat|curl|ps|top|df|du)\s+/i) || 
                       line.match(/^[\$>]\s*(kubectl|kw|docker|helm)/i) ||
                       (line.startsWith('kw ') && line.length > 5) ||
                       line.match(/^\s*(kubectl|kw)\s+/) ||
                       line.includes('```bash') ||
                       line.includes('```sh')
      
      if (isCommand && !inCodeBlock) {
        // Start a new code block
        formatted += '\n```bash\n'
        inCodeBlock = true
        codeBlockContent = originalLine + '\n'
      } else if (inCodeBlock) {
        // Continue the code block
        codeBlockContent += originalLine + '\n'
        
        // Check if we should end the code block (empty line or non-command)
        const nextLine = i + 1 < lines.length ? lines[i + 1].trim() : ''
        if (line === '' || (!isCommand && nextLine === '')) {
          formatted += codeBlockContent
          formatted += '```\n\n'
          inCodeBlock = false
          codeBlockContent = ''
        }
      } else {
        // Regular text
        formatted += originalLine + '\n'
      }
    }
    
    // Close any open code block
    if (inCodeBlock) {
      formatted += codeBlockContent
      formatted += '```\n'
    }
    
    return formatted.trim()
  }

  const usePlaybook = (playbook: data.AIPlaybook, setActiveTab?: (tab: string) => void) => {
    if (setActiveTab && playbook.target === 'Deployment') {
      setActiveTab('deploy')
    }
    sendPrompt(playbook.prompt)
  }

  return {
    draftPrompt,
    chatMessages,
    isThinking,
    sendPrompt,
    usePlaybook,
    formatError,
    formatAIResponse,
  }
}
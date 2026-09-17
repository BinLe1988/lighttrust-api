import { useState, useEffect, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { BookOpen, FlaskConical, BarChart3, PenLine, Code2, ScrollText, Send, Sparkles, Zap } from 'lucide-react'
import { api } from '@/lib/api'

const STAGES = [
  { key: 'literature', name: 'Literature Review', icon: BookOpen },
  { key: 'experiment', name: 'Experiment Design', icon: FlaskConical },
  { key: 'analysis', name: 'Data Analysis', icon: BarChart3 },
  { key: 'writing', name: 'Paper Writing', icon: PenLine },
  { key: 'coding', name: 'Code Implementation', icon: Code2 },
  { key: 'review', name: 'Review Writing', icon: ScrollText },
]

export function ResearchWorkbench() {
  const [stages, setStages] = useState<any[]>([])
  const [currentStage, setCurrentStage] = useState('')
  const [stageConfig, setStageConfig] = useState<any>(null)
  const [messages, setMessages] = useState<any[]>([])
  const [inputValue, setInputValue] = useState('')
  const [loading, setLoading] = useState(false)
  const [streamContent, setStreamContent] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    api.get('/api/research/stages')
      .then(r => r.data)
      .then(d => {
        if (d.success && d.data.length > 0) {
          setStages(d.data)
          handleStageChange(d.data[0].key)
        }
      })
      .catch(() => {})
  }, [])

  const handleStageChange = async (key: string) => {
    setCurrentStage(key)
    setMessages([])
    setStreamContent('')
    try {
      const r = await api.get('/api/research/stages/' + key)
      const d = r.data
      if (d.success) setStageConfig(d.data)
    } catch (e) {}
  }

  const handleSend = async () => {
    const trimmed = inputValue.trim()
    if (!trimmed || loading) return
    const userMsg = { role: 'user', content: trimmed }
    const newMsgs = [...messages, userMsg]
    setMessages(newMsgs)
    setInputValue('')
    setLoading(true)
    setStreamContent('')
    try {
      const r = await api.post('/api/research/chat', {
        stage: currentStage,
        messages: newMsgs.map((m: any) => ({ role: m.role, content: m.content })),
        stream: true,
      }, {
        responseType: 'text',
        onDownloadProgress: (progressEvent: any) => {
          const chunk = progressEvent.event?.target?.response || progressEvent.currentTarget?.response || ''
          const lines = chunk.split('\n')
          let full = ''
          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.slice(6)
              if (data === '[DONE]') continue
              try {
                const p = JSON.parse(data)
                const content = p.choices?.[0]?.delta?.content || ''
                full += content
              } catch (e) {}
            }
          }
          if (full) setStreamContent(prev => prev + full)
        },
      })
      const fullContent = typeof r.data === 'string' ? r.data : ''
      setMessages([...newMsgs, { role: 'assistant', content: fullContent || streamContent }])
      setStreamContent('')
    } catch (e) {}
    finally { setLoading(false) }
  }

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages, streamContent])

  const StageIcon = STAGES.find(s => s.key === currentStage)?.icon || BookOpen

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <div className="flex items-center justify-between px-6 py-3 border-b">
        <div className="flex items-center gap-3">
          <Sparkles className="h-5 w-5 text-primary" />
          <h2 className="text-lg font-semibold">Research Workbench</h2>
          <Badge variant="secondary" className="gap-1">
            <Zap className="h-3 w-3" />
            Token Saver
          </Badge>
        </div>
        <Select value={currentStage} onValueChange={handleStageChange}>
          <SelectTrigger className="w-56">
            <SelectValue placeholder="Select stage" />
          </SelectTrigger>
          <SelectContent>
            {stages.map((s: any) => {
              const Icon = STAGES.find(x => x.key === s.key)?.icon || BookOpen
              return (
                <SelectItem key={s.key} value={s.key}>
                  <Icon className="h-4 w-4 inline mr-2" />
                  {s.name}
                </SelectItem>
              )
            })}
          </SelectContent>
        </Select>
      </div>
      {stageConfig && (
        <div className="px-6 py-2 text-sm text-muted-foreground border-b flex items-center gap-2">
          <span>{stageConfig.description}</span>
          <Badge variant="outline" className="text-xs">{stageConfig.model_prompt}</Badge>
        </div>
      )}
      <ScrollArea className="flex-1 p-6" ref={scrollRef}>
        {messages.length === 0 && !streamContent && (
          <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
            <StageIcon className="h-16 w-16 mb-4 opacity-30" />
            <h3 className="text-lg font-medium">{stageConfig?.name || 'Research Workbench'}</h3>
            <p className="text-sm max-w-md text-center mt-2">
              {stageConfig?.description || 'Select a research stage to begin.'}
            </p>
          </div>
        )}
        {messages.map((msg: any, i: number) => (
          <div
            key={i}
            className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'} mb-4`}
          >
            <div
              className={`max-w-[80%] rounded-xl px-4 py-3 ${
                msg.role === 'user'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted'
              }`}
            >
              <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
            </div>
          </div>
        ))}
        {streamContent && (
          <div className="flex justify-start mb-4">
            <div className="max-w-[80%] rounded-xl px-4 py-3 bg-muted">
              <p className="text-sm whitespace-pre-wrap">{streamContent}</p>
            </div>
          </div>
        )}
      </ScrollArea>
      <div className="px-6 py-4 border-t">
        <div className="flex gap-3">
          <Input
            value={inputValue}
            onChange={e => setInputValue(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                handleSend()
              }
            }}
            placeholder={stageConfig ? 'Ask about ' + stageConfig.name + '...' : 'Select a stage first'}
            disabled={!currentStage || loading}
            className="flex-1"
          />
          <Button
            onClick={handleSend}
            disabled={!inputValue.trim() || !currentStage || loading}
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
        {stageConfig && (
          <div className="flex items-center gap-2 mt-2 text-xs text-muted-foreground">
            <span>Strategy: {stageConfig.strategy}</span>
            <Badge variant="secondary" className="text-xs">Auto-select best model</Badge>
          </div>
        )}
      </div>
    </div>
  )
}

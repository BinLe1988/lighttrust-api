import React, { useState, useEffect, useRef, useContext } from 'react';
import { useTranslation } from 'react-i18next';
import { Layout, Select, Button, Input, Typography, Tag, Spin } from '@douyinfe/semi-ui';
import { BookOpen, FlaskConical, BarChart3, PenLine, Code2, ScrollText, Send, Sparkles, Zap } from 'lucide-react';
import { UserContext } from '../../context/User';
import { API, showError } from '../../helpers';

const { Header, Content } = Layout;
const { Text, Title, Paragraph } = Typography;

const STAGE_ICONS = {
  literature: BookOpen,
  experiment: FlaskConical,
  analysis: BarChart3,
  writing: PenLine,
  coding: Code2,
  review: ScrollText,
};

const Research = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const [stages, setStages] = useState([]);
  const [currentStage, setCurrentStage] = useState('');
  const [stageConfig, setStageConfig] = useState(null);
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const [streamContent, setStreamContent] = useState('');
  const chatRef = useRef(null);

  useEffect(() => { loadStages(); }, []);

  const loadStages = async () => {
    try {
      const res = await API.get('/api/research/stages');
      const { success, data } = res.data;
      if (success && data.length > 0) {
        setStages(data);
        if (!currentStage) handleStageChange(data[0].key);
      }
    } catch (error) { showError('Failed to load research stages'); }
  };

  const handleStageChange = async (stageKey) => {
    setCurrentStage(stageKey);
    setMessages([]);
    setStreamContent('');
    try {
      const res = await API.get('/api/research/stages/' + stageKey);
      const { success, data } = res.data;
      if (success) setStageConfig(data);
    } catch (error) { showError('Failed to load stage config'); }
  };

  const handleSend = async () => {
    if (!inputValue.trim() || loading) return;
    const userMessage = { role: 'user', content: inputValue.trim() };
    const newMessages = [...messages, userMessage];
    setMessages(newMessages);
    setInputValue('');
    setLoading(true);
    setStreaming(true);
    setStreamContent('');
    try {
      const token = localStorage.getItem('token');
      const response = await fetch('/api/research/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + token,
        },
        body: JSON.stringify({
          stage: currentStage,
          messages: newMessages.map(function(m) {
            return { role: m.role, content: m.content };
          }),
          stream: true,
        }),
      });
      if (!response.ok) throw new Error('Request failed');
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let fullContent = '';
      while (true) {
        const result = await reader.read();
        if (result.done) break;
        const chunk = decoder.decode(result.value, { stream: true });
        const lines = chunk.split('\n');
        for (var i = 0; i < lines.length; i++) {
          var line = lines[i];
          if (line.startsWith('data: ')) {
            var data = line.slice(6);
            if (data === '[DONE]') continue;
            try {
              var parsed = JSON.parse(data);
              var content = parsed.choices && parsed.choices[0] && parsed.choices[0].delta && parsed.choices[0].delta.content;
              if (content) {
                fullContent += content;
                setStreamContent(fullContent);
              }
            } catch (e) {}
          }
        }
      }
      setMessages([...newMessages, { role: 'assistant', content: fullContent }]);
      setStreamContent('');
    } catch (error) {
      showError('Chat request failed: ' + error.message);
    } finally {
      setLoading(false);
      setStreaming(false);
    }
  };

  var handleKeyDown = function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  useEffect(function() {
    if (chatRef.current) {
      chatRef.current.scrollTop = chatRef.current.scrollHeight;
    }
  }, [messages, streamContent]);

  var StageIcon = currentStage ? (STAGE_ICONS[currentStage] || BookOpen) : BookOpen;

  return (
    <Layout style={{ height: 'calc(100vh - 60px)' }}>
      <Header style={{
        padding: '12px 24px',
        borderBottom: '1px solid var(--semi-color-border)',
        backgroundColor: 'var(--semi-color-bg-0)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Sparkles size={24} color="var(--semi-color-primary)" />
            <Title heading={4} style={{ margin: 0 }}>Research Workbench</Title>
            <Tag color="light-blue" size="small">
              <Zap size={12} style={{ marginRight: 4 }} />
              Token Saver
            </Tag>
          </div>
          <Select
            value={currentStage}
            onChange={handleStageChange}
            style={{ width: 240 }}
            placeholder="Select research stage"
            optionList={stages.map(function(s) {
              var IconComp = STAGE_ICONS[s.key] || BookOpen;
              return {
                value: s.key,
                label: (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <IconComp size={16} />
                    <span>{s.name}</span>
                  </div>
                ),
              };
            })}
          />
        </div>
        {stageConfig && (
          <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 12 }}>
            <Text type="tertiary" size="small">{stageConfig.description}</Text>
            <Tag color="light-blue" size="small">{stageConfig.model_prompt}</Tag>
          </div>
        )}
      </Header>
      <Content style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
        <div ref={chatRef} style={{ flex: 1, overflow: 'auto', padding: '24px' }}>
          {messages.length === 0 && !streaming && (
            <div style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
              opacity: 0.6,
            }}>
              <StageIcon size={64} strokeWidth={1} />
              <Title heading={5} style={{ marginTop: 16 }}>
                {stageConfig ? stageConfig.name : 'Research Workbench'}
              </Title>
              <Paragraph style={{ maxWidth: 400, textAlign: 'center' }}>
                {stageConfig ? stageConfig.description : 'Select a research stage to begin.'}
              </Paragraph>
            </div>
          )}
          {messages.map(function(msg, idx) {
            var isUser = msg.role === 'user';
            return (
              <div key={idx} style={{
                display: 'flex',
                justifyContent: isUser ? 'flex-end' : 'flex-start',
                marginBottom: 16,
              }}>
                <div style={{
                  maxWidth: '80%',
                  padding: '12px 16px',
                  borderRadius: 12,
                  backgroundColor: isUser ? 'var(--semi-color-primary)' : 'var(--semi-color-fill-0)',
                  color: isUser ? 'var(--semi-color-white)' : 'var(--semi-color-text-0)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}>
                  <Text>{msg.content}</Text>
                </div>
              </div>
            );
          })}
          {streaming && streamContent && (
            <div style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 16 }}>
              <div style={{
                maxWidth: '80%',
                padding: '12px 16px',
                borderRadius: 12,
                backgroundColor: 'var(--semi-color-fill-0)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}>
                <Text>{streamContent}</Text>
                <Spin size="small" style={{ marginLeft: 8, display: 'inline-block' }} />
              </div>
            </div>
          )}
        </div>
        <div style={{
          padding: '16px 24px',
          borderTop: '1px solid var(--semi-color-border)',
          backgroundColor: 'var(--semi-color-bg-0)',
        }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end' }}>
            <Input
              value={inputValue}
              onChange={setInputValue}
              onKeyDown={handleKeyDown}
              placeholder={stageConfig ? 'Ask about ' + stageConfig.name + '...' : 'Select a stage first'}
              disabled={!currentStage || loading}
              style={{ flex: 1 }}
              autosize={{ minRows: 1, maxRows: 4 }}
            />
            <Button
              theme="solid"
              icon={<Send size={16} />}
              onClick={handleSend}
              loading={loading}
              disabled={!inputValue.trim() || !currentStage}
            >
              Send
            </Button>
          </div>
          {stageConfig && (
            <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
              <Text type="tertiary" size="small">
                Strategy: {stageConfig.strategy}
              </Text>
              <Tag size="small" color="light-green">Auto-select best model</Tag>
            </div>
          )}
        </div>
      </Content>
    </Layout>
  );
};

export default Research;

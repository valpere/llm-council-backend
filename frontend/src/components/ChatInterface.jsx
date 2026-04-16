import { useState, useEffect, useRef } from 'react';
import Markdown from './Markdown';
import Stage1 from './Stage1';
import Stage2 from './Stage2';
import Stage3 from './Stage3';
import EmptyState from './EmptyState';
import './ChatInterface.css';

export default function ChatInterface({
  conversation,
  onSendMessage,
  isLoading,
  sidebarOpen,
  onToggleSidebar,
}) {
  const [input, setInput] = useState('');
  const messagesEndRef = useRef(null);
  const textareaRef = useRef(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [conversation]);

  const handleSubmit = (e) => {
    e.preventDefault();
    const text = input.trim();
    if (text && !isLoading) {
      // Create new conversation if none is active — handled upstream; guard here
      onSendMessage(text);
      setInput('');
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto';
      }
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(e);
    }
  };

  const handleInput = (e) => {
    setInput(e.target.value);
    e.target.style.height = 'auto';
    e.target.style.height = `${Math.min(e.target.scrollHeight, 200)}px`;
  };

  if (!conversation) {
    return (
      <div className="chat-interface">
        <div className="chat-header">
          {!sidebarOpen && (
            <button className="sidebar-open-btn" onClick={onToggleSidebar} aria-label="Open sidebar">
              ☰
            </button>
          )}
        </div>
        <div className="no-conversation">
          <p>Select or create a conversation to get started</p>
        </div>
      </div>
    );
  }

  return (
    <div className="chat-interface">
      <div className="chat-header">
        {!sidebarOpen && (
          <button className="sidebar-open-btn" onClick={onToggleSidebar} aria-label="Open sidebar">
            ☰
          </button>
        )}
        {conversation.title && (
          <span className="chat-title">{conversation.title}</span>
        )}
      </div>

      <div className="messages-container">
        {conversation.messages.length === 0 ? (
          <EmptyState onSendMessage={onSendMessage} isLoading={isLoading} />
        ) : (
          conversation.messages.map((msg, index) => (
            <div key={index} className="message-group">
              {msg.role === 'user' ? (
                <div className="user-message">
                  <div className="message-label">You</div>
                  <div className="message-content">
                    <div className="markdown-content">
                      <Markdown>{msg.content}</Markdown>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="assistant-message">
                  <div className="message-label">LLM Council</div>

                  {/* Stage 1 */}
                  <Stage1
                    responses={msg.stage1}
                    isLoading={msg.loading?.stage1}
                  />

                  {/* Stage 2 */}
                  <Stage2
                    rankings={msg.stage2}
                    labelToModel={msg.metadata?.label_to_model}
                    aggregateRankings={msg.metadata?.aggregate_rankings}
                    consensusW={msg.metadata?.consensus_w}
                    isLoading={msg.loading?.stage2}
                  />

                  {/* Stage 3 */}
                  {msg.loading?.stage3 && (
                    <div className="stage-loading">
                      <div className="spinner"></div>
                      <span>Synthesising final answer...</span>
                    </div>
                  )}
                  {(msg.stage3 || msg.error) && (
                    <Stage3 finalResponse={msg.stage3} error={msg.error} />
                  )}
                </div>
              )}
            </div>
          ))
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input is always visible when a conversation is active */}
      <form className="input-form" onSubmit={handleSubmit}>
        <textarea
          ref={textareaRef}
          className="message-input"
          placeholder="Ask a question… (Enter to send, Shift+Enter for new line)"
          value={input}
          onInput={handleInput}
          onKeyDown={handleKeyDown}
          disabled={isLoading}
          rows={1}
        />
        <button
          type="submit"
          className="send-button"
          disabled={!input.trim() || isLoading}
        >
          Send
        </button>
      </form>
    </div>
  );
}

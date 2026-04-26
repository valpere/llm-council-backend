import { useState, useEffect, useRef } from 'react';
import Markdown from './Markdown';
import { stripMarkdown } from '../utils';

import Stage0 from './Stage0';
import Stage1 from './Stage1';
import Stage2 from './Stage2';
import Stage3 from './Stage3';
import EmptyState from './EmptyState';
import './ChatInterface.css';

export default function ChatInterface({
  conversation,
  onSendMessage,
  onAnswerSubmit,
  isLoading,
  sidebarOpen,
  onToggleSidebar,
}) {
  const [input, setInput] = useState('');
  const messagesContainerRef = useRef(null);
  const textareaRef = useRef(null);

  useEffect(() => {
    const el = messagesContainerRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [conversation?.messages]);

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
          <span className="chat-title">{stripMarkdown(conversation.title)}</span>
        )}
      </div>

      <div className="messages-container" ref={messagesContainerRef}>
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

                  {/* Stage 0 */}
                  <Stage0
                    pendingClarification={msg.pendingClarification}
                    isLoading={msg.loading?.stage0}
                    onSubmit={onAnswerSubmit}
                  />

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

      </div>

      {/* Input is always visible when a conversation is active */}
      <form className="input-form" onSubmit={handleSubmit}>
        <textarea
          ref={textareaRef}
          className="message-input"
          placeholder={
            conversation.messages.at(-1)?.pendingClarification
              ? 'Answer the questions above to continue…'
              : 'Ask a question… (Enter to send, Shift+Enter for new line)'
          }
          value={input}
          onInput={handleInput}
          onKeyDown={handleKeyDown}
          disabled={isLoading || !!conversation.messages.at(-1)?.pendingClarification}
          rows={1}
        />
        <button
          type="submit"
          className="send-button"
          disabled={!input.trim() || isLoading || !!conversation.messages.at(-1)?.pendingClarification}
        >
          Send
        </button>
      </form>
    </div>
  );
}

import { useState, useEffect } from 'react';
import Sidebar from './components/Sidebar';
import ChatInterface from './components/ChatInterface';
import { api } from './api';
import './App.css';

function App() {
  const [conversations, setConversations] = useState([]);
  const [currentConversationId, setCurrentConversationId] = useState(null);
  const [currentConversation, setCurrentConversation] = useState(null);
  const [isLoading, setIsLoading] = useState(false);

  const loadConversations = async () => {
    try {
      const convs = await api.listConversations();
      setConversations(convs);
    } catch (error) {
      console.error('Failed to load conversations:', error);
    }
  };

  const loadConversation = async (id) => {
    try {
      const conv = await api.getConversation(id);
      setCurrentConversation(conv);
    } catch (error) {
      console.error('Failed to load conversation:', error);
    }
  };

  // Load conversations on mount
  useEffect(() => {
    loadConversations();
  }, []);

  // Load conversation details when selected
  useEffect(() => {
    if (currentConversationId) {
      loadConversation(currentConversationId);
    }
  }, [currentConversationId]);

  const handleNewConversation = async () => {
    try {
      const newConv = await api.createConversation();
      setConversations((prev) => [
        { id: newConv.id, created_at: newConv.created_at, message_count: 0 },
        ...prev,
      ]);
      setCurrentConversationId(newConv.id);
    } catch (error) {
      console.error('Failed to create conversation:', error);
    }
  };

  const handleSelectConversation = (id) => {
    setCurrentConversationId(id);
  };

  const handleSendMessage = async (content) => {
    if (!currentConversationId) return;

    setIsLoading(true);
    try {
      // Optimistically add user message to UI
      const userMessage = { role: 'user', content };
      setCurrentConversation((prev) => ({
        ...prev,
        messages: [...prev.messages, userMessage],
      }));

      // Create a partial assistant message that will be updated progressively
      const assistantMessage = {
        role: 'assistant',
        stage1: null,
        stage2: null,
        stage3: null,
        metadata: null,
        loading: {
          stage1: false,
          stage2: false,
          stage3: false,
        },
        error: null,
      };

      // Add the partial assistant message
      setCurrentConversation((prev) => ({
        ...prev,
        messages: [...prev.messages, assistantMessage],
      }));

      const updateLast = (updater) =>
        setCurrentConversation((prev) => {
          const messages = [...prev.messages];
          const last = { ...messages[messages.length - 1], loading: { ...messages[messages.length - 1].loading } };
          messages[messages.length - 1] = last;
          updater(last);
          return { ...prev, messages };
        });

      const sseHandlers = {
        stage1_start: () => updateLast((msg) => { msg.loading.stage1 = true; }),
        stage1_complete: (event) => updateLast((msg) => {
          msg.stage1 = event.data;
          msg.loading.stage1 = false;
        }),
        stage2_start: () => updateLast((msg) => { msg.loading.stage2 = true; }),
        stage2_complete: (event) => updateLast((msg) => {
          msg.stage2 = event.data;
          msg.metadata = event.metadata;
          msg.loading.stage2 = false;
        }),
        stage3_start: () => updateLast((msg) => { msg.loading.stage3 = true; }),
        stage3_complete: (event) => updateLast((msg) => {
          msg.stage3 = event.data;
          msg.loading.stage3 = false;
        }),
        title_complete: () => loadConversations(),
        complete: () => { loadConversations(); setIsLoading(false); },
        error: (event) => {
          updateLast((msg) => { msg.error = event.message; msg.loading.stage3 = false; });
          setIsLoading(false);
        },
      };

      // Send message with streaming
      await api.sendMessageStream(currentConversationId, content, (eventType, event) => {
        if (Object.hasOwn(sseHandlers, eventType)) {
          sseHandlers[eventType](event);
        } else {
          console.log('Unknown event type:', eventType);
        }
      });
    } catch (error) {
      console.error('Failed to send message:', error);
      // Remove optimistic messages on error
      setCurrentConversation((prev) => ({
        ...prev,
        messages: prev.messages.slice(0, -2),
      }));
      setIsLoading(false);
    }
  };

  return (
    <div className="app">
      <Sidebar
        conversations={conversations}
        currentConversationId={currentConversationId}
        onSelectConversation={handleSelectConversation}
        onNewConversation={handleNewConversation}
      />
      <ChatInterface
        conversation={currentConversation}
        onSendMessage={handleSendMessage}
        isLoading={isLoading}
      />
    </div>
  );
}

export default App;

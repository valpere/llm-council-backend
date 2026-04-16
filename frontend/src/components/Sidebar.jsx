import './Sidebar.css';

function formatDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export default function Sidebar({
  conversations,
  currentConversationId,
  onSelectConversation,
  onNewConversation,
  isOpen,
  onToggle,
  theme,
  onToggleTheme,
}) {
  return (
    <div className={`sidebar${isOpen ? '' : ' collapsed'}`}>
      <div className="sidebar-header">
        <span className="sidebar-title">LLM Council</span>
        <button className="sidebar-toggle" onClick={onToggle} title="Toggle sidebar">
          {isOpen ? '‹' : '›'}
        </button>
      </div>

      <div className="sidebar-body">
        <button className="new-conversation-btn" onClick={onNewConversation}>
          + New Conversation
        </button>

        <div className="conversation-list">
          {conversations.length === 0 ? (
            <div className="no-conversations">No conversations yet</div>
          ) : (
            conversations.map((conv) => (
              <div
                key={conv.id}
                className={`conversation-item${conv.id === currentConversationId ? ' active' : ''}`}
                onClick={() => onSelectConversation(conv.id)}
              >
                <div className="conversation-title">
                  {conv.title || 'New Conversation'}
                </div>
                <div className="conversation-meta">
                  {formatDate(conv.created_at)}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      <div className="sidebar-footer">
        <button className="theme-toggle" onClick={onToggleTheme} title="Toggle theme">
          {theme === 'dark' ? '☀' : '☾'}
        </button>
      </div>
    </div>
  );
}

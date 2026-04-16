import Markdown from './Markdown';
import './Stage3.css';

export default function Stage3({ finalResponse, error }) {
  if (!finalResponse && !error) {
    return null;
  }

  return (
    <div className="stage3-hero">
      <div className="stage3-header">
        <span className="stage3-label">Final Answer</span>
        {finalResponse && (
          <span className="stage3-model">
            {finalResponse.model.split('/')[1] || finalResponse.model}
          </span>
        )}
      </div>

      {error ? (
        <div className="stage3-error">
          <span className="stage3-error-icon">⚠</span>
          <span className="stage3-error-message">{error}</span>
        </div>
      ) : (
        <div className="final-text markdown-content">
          <Markdown>{finalResponse.content}</Markdown>
        </div>
      )}
    </div>
  );
}

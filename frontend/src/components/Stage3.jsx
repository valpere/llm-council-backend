import ReactMarkdown from 'react-markdown';
import { modelShortName } from '../utils';
import './Stage3.css';

export default function Stage3({ finalResponse, error }) {
  if (!finalResponse && !error) {
    return null;
  }

  return (
    <div className="stage stage3">
      <h3 className="stage-title">Stage 3: Final Council Answer</h3>
      {error ? (
        <div className="stage3-error">
          <span className="stage3-error-icon">⚠</span>
          <span className="stage3-error-message">{error}</span>
        </div>
      ) : (
        <div className="final-response">
          <div className="chairman-label">
            Chairman: {modelShortName(finalResponse.model)}
          </div>
          <div className="final-text markdown-content">
            <ReactMarkdown>{finalResponse.response}</ReactMarkdown>
          </div>
        </div>
      )}
    </div>
  );
}

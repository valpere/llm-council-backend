import { useState } from 'react';
import Markdown from './Markdown';
import './Stage1.css';

export default function Stage1({ responses, isLoading }) {
  const [expanded, setExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState(0);

  if (!isLoading && (!responses || responses.length === 0)) {
    return null;
  }

  const count = responses?.length ?? 0;

  return (
    <div className="stage stage1">
      <button
        className="stage-accordion"
        onClick={() => setExpanded((e) => !e)}
        aria-expanded={expanded}
      >
        <span className="stage-accordion-label">
          {isLoading ? (
            <>
              <span className="spinner spinner-sm" />
              Collecting individual responses…
            </>
          ) : (
            `Stage 1: Individual Responses (${count} model${count !== 1 ? 's' : ''})`
          )}
        </span>
        {!isLoading && (
          <span className="stage-accordion-chevron">{expanded ? '▲' : '▼'}</span>
        )}
      </button>

      {expanded && responses && responses.length > 0 && (
        <div className="stage-body">
          <div className="tabs">
            {responses.map((resp, index) => (
              <button
                key={index}
                className={`tab${activeTab === index ? ' active' : ''}`}
                onClick={() => setActiveTab(index)}
              >
                {resp.model.split('/')[1] || resp.model}
              </button>
            ))}
          </div>

          <div className="tab-content">
            <div className="model-name">{responses[activeTab].model}</div>
            <div className="response-text markdown-content">
              <Markdown>{responses[activeTab].content}</Markdown>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

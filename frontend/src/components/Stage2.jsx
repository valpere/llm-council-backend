import { useState } from 'react';
import './Stage2.css';

function modelShortName(model) {
  return model.split('/')[1] || model;
}

function consensusLabel(w) {
  if (w >= 0.70) return 'strong';
  if (w >= 0.40) return 'moderate';
  return 'weak';
}

export default function Stage2({ rankings, labelToModel, aggregateRankings, consensusW, isLoading }) {
  const [expanded, setExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState(0);

  if (!isLoading && (!rankings || rankings.length === 0)) {
    return null;
  }

  const hasConsensus = consensusW != null && consensusW > 0;
  const label = hasConsensus ? consensusLabel(consensusW) : null;

  return (
    <div className="stage stage2">
      <button
        className="stage-accordion"
        onClick={() => setExpanded((e) => !e)}
        aria-expanded={expanded}
      >
        <span className="stage-accordion-label">
          {isLoading ? (
            <>
              <span className="spinner-sm" />
              Running peer rankings…
            </>
          ) : (
            <>
              Stage 2: Peer Rankings
              {hasConsensus && (
                <span className={`consensus-pill consensus-${label}`}>
                  W={consensusW.toFixed(2)} {label}
                </span>
              )}
            </>
          )}
        </span>
        {!isLoading && (
          <span className="stage-accordion-chevron">{expanded ? '▲' : '▼'}</span>
        )}
      </button>

      {expanded && rankings && rankings.length > 0 && (
        <div className="stage-body">
          <div className="tabs">
            {rankings.map((rank, index) => {
              const reviewerModel = labelToModel?.[rank.reviewer_label] ?? rank.reviewer_label;
              return (
                <button
                  key={index}
                  className={`tab${activeTab === index ? ' active' : ''}`}
                  onClick={() => setActiveTab(index)}
                >
                  {modelShortName(reviewerModel)}
                </button>
              );
            })}
          </div>

          <div className="tab-content">
            <div className="model-name">
              {labelToModel?.[rankings[activeTab].reviewer_label] ?? rankings[activeTab].reviewer_label}
            </div>
            {rankings[activeTab].rankings && rankings[activeTab].rankings.length > 0 ? (
              <div className="parsed-ranking">
                <strong>Ranking (best → worst):</strong>
                <ol>
                  {rankings[activeTab].rankings.map((lbl, i) => (
                    <li key={i}>
                      {labelToModel?.[lbl] ? modelShortName(labelToModel[lbl]) : lbl}
                    </li>
                  ))}
                </ol>
              </div>
            ) : (
              <p className="ranking-missing">No rankings submitted by this reviewer.</p>
            )}
          </div>

          {aggregateRankings && aggregateRankings.length > 0 && (
            <div className="aggregate-rankings">
              <div className="aggregate-title">Aggregate Rankings</div>
              <div className="aggregate-list">
                {aggregateRankings.map((agg, index) => (
                  <div key={index} className="aggregate-item">
                    <span className="rank-position">#{index + 1}</span>
                    <span className="rank-model">{modelShortName(agg.model)}</span>
                    <span className="rank-score">{agg.score.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

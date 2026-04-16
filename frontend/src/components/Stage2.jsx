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

export default function Stage2({ rankings, labelToModel, aggregateRankings, consensusW }) {
  const [activeTab, setActiveTab] = useState(0);

  if (!rankings || rankings.length === 0) {
    return null;
  }

  return (
    <div className="stage stage2">
      <h3 className="stage-title">Stage 2: Peer Rankings</h3>

      {consensusW != null && consensusW > 0 && (
        <div className={`consensus-badge consensus-${consensusLabel(consensusW)}`}>
          Consensus: {consensusW.toFixed(2)} ({consensusLabel(consensusW)})
        </div>
      )}

      <h4>Individual Rankings</h4>
      <p className="stage-description">
        Each model ranked all responses (anonymized as Response A, B, C, etc.).
        Model names shown below are resolved via the label map.
      </p>

      <div className="tabs">
        {rankings.map((rank, index) => {
          const reviewerModel = labelToModel?.[rank.reviewer_label] ?? rank.reviewer_label;
          return (
            <button
              key={index}
              className={`tab ${activeTab === index ? 'active' : ''}`}
              onClick={() => setActiveTab(index)}
            >
              {modelShortName(reviewerModel)}
            </button>
          );
        })}
      </div>

      <div className="tab-content">
        <div className="ranking-model">
          {labelToModel?.[rankings[activeTab].reviewer_label] ?? rankings[activeTab].reviewer_label}
        </div>
        {rankings[activeTab].rankings && rankings[activeTab].rankings.length > 0 ? (
          <div className="parsed-ranking">
            <strong>Ranking (best to worst):</strong>
            <ol>
              {rankings[activeTab].rankings.map((label, i) => (
                <li key={i}>
                  {labelToModel?.[label] ? modelShortName(labelToModel[label]) : label}
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
          <h4>Aggregate Rankings</h4>
          <p className="stage-description">
            Combined results across all peer evaluations (lower score is better):
          </p>
          <div className="aggregate-list">
            {aggregateRankings.map((agg, index) => (
              <div key={index} className="aggregate-item">
                <span className="rank-position">#{index + 1}</span>
                <span className="rank-model">{modelShortName(agg.model)}</span>
                <span className="rank-score">Score: {agg.score.toFixed(2)}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

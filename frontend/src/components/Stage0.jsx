import { useState } from 'react';
import Markdown from './Markdown';
import './Stage0.css';

export default function Stage0({ pendingClarification, isLoading, onSubmit }) {
  const [draftAnswers, setDraftAnswers] = useState({});

  if (!isLoading && !pendingClarification) return null;

  const handleTextChange = (id, text) => {
    setDraftAnswers((prev) => ({ ...prev, [id]: text }));
  };

  const buildAnswers = () =>
    (pendingClarification?.questions ?? []).map((q) => ({
      id: q.id,
      text: draftAnswers[q.id] ?? '',
    }));

  const handleSubmit = () => {
    const answers = buildAnswers();
    setDraftAnswers({});
    onSubmit(answers);
  };

  const handleSkip = () => {
    setDraftAnswers({});
    onSubmit(buildAnswers().map((a) => ({ ...a, text: '' })));
  };

  return (
    <>
      {isLoading && (
        <div className="stage0-loading">
          <span className="spinner-sm" />
          Identifying clarification questions…
        </div>
      )}

      {pendingClarification && (
        <div className="stage0-pending">
          <div className="stage0-round-label">
            Clarification questions (round {pendingClarification.round})
          </div>
          {pendingClarification.questions.map((q) => (
            <div key={q.id} className="stage0-question">
              <div className="stage0-question-text">
                <Markdown>{q.text}</Markdown>
              </div>
              <textarea
                className="stage0-answer-input"
                placeholder="Your answer (optional)"
                value={draftAnswers[q.id] ?? ''}
                onChange={(e) => handleTextChange(q.id, e.target.value)}
                rows={2}
              />
            </div>
          ))}
          <div className="stage0-actions">
            <button className="stage0-submit-btn" onClick={handleSubmit}>
              Submit answers
            </button>
            <button className="stage0-skip-btn" onClick={handleSkip}>
              Skip
            </button>
          </div>
        </div>
      )}
    </>
  );
}

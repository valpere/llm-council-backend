/**
 * Returns the short display name for a model identifier.
 * e.g. "openai/gpt-4o" → "gpt-4o", "gpt-4o" → "gpt-4o"
 */
export function modelShortName(model) {
  return model.split('/')[1] || model;
}

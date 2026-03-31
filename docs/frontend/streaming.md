# Streaming Protocol (SSE)

`POST /api/conversations/{id}/message/stream` returns a `text/event-stream` response. Events arrive as the 3-stage pipeline progresses.

## Event Format

Each event is a single line:

```
data: <JSON>\n\n
```

The JSON object always has a `type` field. Additional fields depend on the event type.

## Event Sequence

```
→  stage1_start
←  stage1_complete   { data: StageOneResult[] }
→  stage2_start
←  stage2_complete   { data: StageTwoResult[], metadata: Metadata }
→  stage3_start
←  stage3_complete   { data: StageThreeResult }
←  title_complete    { data: { title: string } }   (may arrive any time after stage1)
←  complete
```

`title_complete` is emitted concurrently with the stage pipeline (title is generated alongside Stage 1). It may arrive before or after `stage3_complete`; the spec allows either ordering and clients must handle both correctly.

## Event Payloads

### stage1_start

```json
{ "type": "stage1_start" }
```

### stage1_complete

```json
{
  "type": "stage1_complete",
  "data": [
    { "model": "openai/gpt-5.1", "response": "..." },
    { "model": "anthropic/claude-sonnet-4.5", "response": "..." }
  ]
}
```

### stage2_start

```json
{ "type": "stage2_start" }
```

### stage2_complete

```json
{
  "type": "stage2_complete",
  "data": [
    {
      "model": "openai/gpt-5.1",
      "ranking": "Full evaluation text with FINAL RANKING:\n1. Response C\n...",
      "parsed_ranking": ["Response C", "Response A", "Response B", "Response D"]
    }
  ],
  "metadata": {
    "label_to_model": {
      "Response A": "openai/gpt-5.1",
      "Response B": "anthropic/claude-sonnet-4.5",
      "Response C": "google/gemini-3-pro-preview",
      "Response D": "x-ai/grok-4"
    },
    "aggregate_rankings": [
      { "model": "google/gemini-3-pro-preview", "average_rank": 1.25, "rankings_count": 4 },
      { "model": "openai/gpt-5.1", "average_rank": 2.0, "rankings_count": 4 }
    ]
  }
}
```

`metadata.label_to_model` maps the anonymized labels used during peer review back to real model names. The frontend uses this to de-anonymize Stage 2 display text.

`metadata.aggregate_rankings` is sorted by `average_rank` ascending (best ranked first).

### stage3_start

```json
{ "type": "stage3_start" }
```

### stage3_complete

```json
{
  "type": "stage3_complete",
  "data": {
    "model": "google/gemini-3-pro-preview",
    "response": "Final synthesized answer..."
  }
}
```

### title_complete

```json
{
  "type": "title_complete",
  "data": { "title": "Why the Sky Appears Blue" }
}
```

The frontend reloads the conversation list on this event to update the sidebar title.

### complete

```json
{ "type": "complete" }
```

Stream is finished. The frontend sets `isLoading = false`.

### error

```json
{ "type": "error", "message": "all council models failed" }
```

Stream terminates after this event. The frontend sets `msg.error = event.message`, stops loading (`loading.stage3 = false`, `isLoading = false`), and no further SSE events are sent.

## Frontend Implementation

`src/api.js` reads the SSE stream using the Fetch API and a `ReadableStream` reader. A line buffer handles SSE events that are split across TCP chunk boundaries:

```javascript
const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = '';

while (true) {
  const { done, value } = await reader.read();

  // On done, flush the decoder; otherwise decode with stream:true to
  // handle multi-byte characters split across chunk boundaries.
  buffer += done ? decoder.decode() : decoder.decode(value, { stream: true });
  const lines = buffer.split('\n');
  // When not done, keep the last (potentially incomplete) line in the
  // buffer. When done, process everything (no more chunks will arrive).
  buffer = done ? '' : lines.pop();

  for (const line of lines) {
    if (line.startsWith('data: ')) {
      const data = line.slice(6);
      try {
        const event = JSON.parse(data);
        onEvent(event.type, event);
      } catch (e) {
        console.error('Failed to parse SSE event:', e);
      }
    }
  }

  if (done) break;
}
```

`App.jsx` maps each `eventType` to a state update:

| Event | State change |
|-------|-------------|
| `stage1_start` | `loading.stage1 = true` |
| `stage1_complete` | `stage1 = event.data`, `loading.stage1 = false` |
| `stage2_start` | `loading.stage2 = true` |
| `stage2_complete` | `stage2 = event.data`, `metadata = event.metadata`, `loading.stage2 = false` |
| `stage3_start` | `loading.stage3 = true` |
| `stage3_complete` | `stage3 = event.data`, `loading.stage3 = false` |
| `title_complete` | reload conversation list |
| `complete` | reload conversation list, `isLoading = false` |
| `error` | `msg.error = event.message`, `loading.stage3 = false`, `isLoading = false` |

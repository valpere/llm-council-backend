---
applyTo: "frontend/**"
---

# Frontend review rules

## Architecture (immutable — flag every violation)

- **No fetch in components.** Any component importing `api.js` or calling `fetch`/`XMLHttpRequest` directly is a violation.
- **No raw HTML injection.** LLM output must go through `<Markdown>` (the `react-markdown` wrapper in `components/Markdown.jsx`). The `dangerouslySetInner​HTML` prop is banned — LLM content is untrusted and must never be injected as raw HTML.
- **State belongs in App.jsx.** Components must not call `setCurrentConversation` or `setConversations`. State flows down via props; events flow up via callbacks.
- **api.js is the sole network boundary.** It returns plain JS values or calls an `onEvent(type, event)` callback. HTTP status codes and raw SSE lines must not leak past this module.

## CSS

- Use `var(--token)` from `theme.css` for all colours, radii, spacing tokens, and the sidebar width. Hardcoded colour values (`#fff`, `rgba(...)`, named colours) are not allowed.
- Each component has a co-located `.css` file. Do not add component styles to `index.css` or `App.css`.

## React

- No TypeScript — plain JS with JSDoc comments only if the type is truly non-obvious.
- No router, no Redux, no Context API. Single-page, sidebar-selection navigation.
- `useEffect` dependency arrays must be complete (ESLint `react-hooks/exhaustive-deps` enforces this).
- Use functional updater form `setState(prev => ...)` when new state depends on previous state.

## Utilities

- Shared helpers go in `src/utils.js`. Do not duplicate logic across components.

## Quality gates

- `npm run lint` must pass with zero errors before merge.
- `npm run build` must succeed (no missing imports, no dead exports).

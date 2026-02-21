# Agent Instructions

## UI Theming Rules
- Do not hardcode colors in page/components (for example in `page.tsx` inline styles).
- Keep shared visual theme values in CSS variables in `ui/app/root.css`.
- Prefer semantic CSS classes in TSX/JSX over inline style objects.
- When adding new UI, extend existing theme CSS files first; only add component-local CSS when necessary.

## Dashboard Styling
- Keep dashboard styling in `ui/app/dashboard.css`.
- If a new dashboard style is needed, add a class and use CSS variables instead of raw color literals in TSX.

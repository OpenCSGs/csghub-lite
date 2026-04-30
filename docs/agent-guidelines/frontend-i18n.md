# Frontend Internationalization

- Any user-facing copy in `web/src` must be localizable.
- Add new strings to `web/src/i18n.ts` in both `en` and `zh`.
- Do not hardcode labels, buttons, tooltips, empty states, errors, status text,
  or helper text directly inside components.
- Use `t()` for UI strings and localized objects like `{ en, zh }` for
  catalog-style content.
- When adding a feature, translate all supporting states too: loading, success,
  failure, empty, confirmation, logs, and hints.
- Keep translation keys stable and grouped by feature prefix.

```tsx
// Bad
<button>Install</button>

// Good
<button>{t("aiApps.install")}</button>
```

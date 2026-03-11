# `pkg/i18n`

Transport-neutral localization support package.

Owns catalog loading and locale-aware lookup primitives reused by HTTP and non-HTTP consumers.

`pkg/i18n` no longer owns a separate root application error type.
The canonical application error contract now lives in `pkg/core/errors`.
This package only keeps compatibility aliases for message and error contracts while owning translation, locale, and catalog primitives.

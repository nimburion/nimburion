# `pkg/config`

Root configuration composition package.

Owns the aggregate `Config` type plus loader/provider orchestration for family-owned config contributors. New section-specific defaults, env binding, validation, and schema ownership must live in the relevant family package, not here.

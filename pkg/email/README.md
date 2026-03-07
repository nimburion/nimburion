# `pkg/email`

Shared email family root.

Owns provider-neutral contracts, message types, family config/feature wiring, and explicit provider assembly. Concrete providers live in provider subpackages such as `smtp`, `ses`, and `sendgrid`.

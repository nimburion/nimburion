# pkg/descriptor

Owns machine-readable service descriptor generation for Nimburion applications.

The descriptor is the stable contract consumed by `nimbctl` for application
identity, command discovery, transport-family metadata, management semantics,
config capabilities, migration metadata, and published artifacts. It is emitted
through the standard CLI `describe` command and does not depend on AST or source
tree heuristics.

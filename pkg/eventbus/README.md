# `pkg/eventbus`

Durable event bus family root.

Owns transport-neutral event bus contracts plus provider packages such as Kafka, RabbitMQ, and SQS. Reliability concerns such as outbox, idempotency, and retry live under `pkg/reliability`, not here.

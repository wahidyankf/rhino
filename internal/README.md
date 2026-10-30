# internal

The Go port's hexagonal layout, mirroring the hippo structural template.

- `domain/` holds closed vocabularies every boundary shares: exit codes, error codes, and JSON rendering.
- `application/` owns the invocation contract: parsing, dispatch, reports, scanning, and validator cores. Its
  `Tree` and `Launcher` interfaces are declared here, at the consumer.
- `adapters/` holds the boundary implementations: CLI streams, configuration, and the read-only disk runtime.
- `bootstrap/` composes production adapters for the command entry point.

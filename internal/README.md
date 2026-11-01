# internal

The Go port's hexagonal layout, mirroring the hippo structural template.

- `domain/` holds closed vocabularies every boundary shares: error codes and JSON rendering.
- `application/` owns the invocation contract: parsing, dispatch, reports, scanning, and validator cores. Its `Tree` and
  `Launcher` interfaces are declared here, at the consumer.
- `adapters/` holds the boundary implementations: CLI streams, configuration, and the disk runtime. The runtime's tree
  walk is read-only; the runtime also performs the declared writes and starts the declared child processes.
- `bootstrap/` composes production adapters for the command entry point.

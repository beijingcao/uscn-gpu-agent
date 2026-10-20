# Security

Please do not put a device key, pairing code, SSH password, production report,
account details, or exploit details into a public issue. Use the private security
advisory form for this repository if enabled, or contact the operator through
the USCN platform support channel.

The agent collects hardware metadata using bounded, fixed read-only commands.
It does not download a GPU driver, execute a model, read user documents, or
accept arbitrary remote commands. The platform may request a fresh inventory
sample; it cannot supply a command to run.

Ed25519 signatures authenticate a paired device key. They are not independent
hardware attestation or proof that SSH access, a GPU workload, or a rental
service is available. Missing reports require investigation and do not by
themselves establish fault or a refund entitlement.

Install from a fixed release and verify SHA256SUMS. The macOS release currently
uses an ad-hoc signature and has not been notarized. The Windows release does
not have an Authenticode signature. Real NVIDIA and Windows service validation
must be distinguished from compilation and fixture-based tests.

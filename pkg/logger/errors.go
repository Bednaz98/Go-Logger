package logger

import "errors"

var (
	errNilStore = errors.New("logger: LocalLogStore is required")

	// ErrNilClient is returned by Init when the client argument is nil.
	ErrNilClient = errors.New("logger: Client is required")

	// ErrNoRemoteTarget is returned by NewClient when remote sending is enabled but neither
	// GRPCAddress nor RemoteURL yields a dial target.
	ErrNoRemoteTarget = errors.New("logger: GRPCAddress or RemoteURL is required when remote sending is enabled")

	// ErrServerClientDisableRemote is returned by NewServerClient when Options.DisableRemote is true.
	// Server-side clients always send over gRPC; use NewDeviceClient with DisableRemote for local-only buffering.
	ErrServerClientDisableRemote = errors.New("logger: ServerClient requires remote ingest (DisableRemote must be false)")
)

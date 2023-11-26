package proto

const (
	// The version of the wraith-module-pinecomms protocol supported by the current
	// version of the module. This is updated whenever a breaking change is made to
	// the protocol.
	//
	// https://web.archive.org/web/20220618041607/https://www.ssa.gov/oact/babynames/decades/century.html
	CURRENT_PROTO = "james"

	// The prefix for all Wraith PineComms HTTP routes.
	ROUTE_PREFIX = "/_wpc/" + CURRENT_PROTO + "/"
)

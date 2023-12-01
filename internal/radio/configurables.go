package radio

const (
	MGMT_PORT_MIN = 10000
	MGMT_PORT_MAX = 50000
	MGMT_ADMIN    = "0000000189de3efe21e7adc6c54d26b80bd875411011dcc3db7c4ab68f162b98"

	DEBUG_SOCKET = "none"
)

var PEERS = []string{
	"tls://0.ygg.l1qu1d.net:11101?key=0000000998b5ff8c0f1115ce9212f772d0427151f50fe858e6de1d22600f1680",
}

var LISTEN = []string{
	"tls://0.0.0.0:0",
	"tls://[::]:0",
}

package symbols

import (
	"go/constant"
	"go/token"
	"reflect"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith/libwraith"
)

var SymbolsLibwraith map[string]map[string]reflect.Value
var SymbolsPc3 map[string]map[string]reflect.Value
var SymbolsProto map[string]map[string]reflect.Value

func init() {
	SymbolsLibwraith = make(map[string]map[string]reflect.Value)
	SymbolsPc3 = make(map[string]map[string]reflect.Value)
	SymbolsProto = make(map[string]map[string]reflect.Value)

	// Generated with `yaegi extract`.

	SymbolsLibwraith["wmp/libwraith/libwraith"] = map[string]reflect.Value{
		"SHMCONF_WATCHER_CHAN_SIZE":     reflect.ValueOf(constant.MakeFromLiteral("255", token.INT, 0)),
		"SHMCONF_WATCHER_NOTIF_TIMEOUT": reflect.ValueOf(constant.MakeFromLiteral("1", token.INT, 0)),
		"SHM_ERRS":                      reflect.ValueOf(constant.MakeFromLiteral("\"err\"", token.STRING, 0)),

		"Config": reflect.ValueOf((*libwraith.Config)(nil)),
		"Wraith": reflect.ValueOf((*libwraith.Wraith)(nil)),
	}

	SymbolsPc3["wmp/pc3/pc3"] = map[string]reflect.Value{
		"CommandContext": reflect.ValueOf((*lib.CommandContext)(nil)),
		"Config":         reflect.ValueOf((*lib.Config)(nil)),
	}

	SymbolsProto["wmp/proto/proto"] = map[string]reflect.Value{
		"CURRENT_PROTO":             reflect.ValueOf(constant.MakeFromLiteral("\"james\"", token.STRING, 0)),
		"HEARTBEAT_INTERVAL_MAX":    reflect.ValueOf(constant.MakeFromLiteral("40", token.INT, 0)),
		"HEARTBEAT_INTERVAL_MIN":    reflect.ValueOf(constant.MakeFromLiteral("20", token.INT, 0)),
		"HEARTBEAT_MARK_DEAD_DELAY": reflect.ValueOf(constant.MakeFromLiteral("81", token.INT, 0)),
		"MarshalRR":                 reflect.ValueOf(proto.Marshal[proto.PacketRR]),
		"MarshalHeartbeat":          reflect.ValueOf(proto.Marshal[proto.PacketHeartbeat]),
		"ROUTE_HEARTBEAT":           reflect.ValueOf(constant.MakeFromLiteral("\"hb\"", token.STRING, 0)),
		"ROUTE_PREFIX":              reflect.ValueOf(constant.MakeFromLiteral("\"/_wpc/james/\"", token.STRING, 0)),
		"ROUTE_REQUEST":             reflect.ValueOf(constant.MakeFromLiteral("\"rq\"", token.STRING, 0)),
		"ROUTE_RESPONSE":            reflect.ValueOf(constant.MakeFromLiteral("\"rs\"", token.STRING, 0)),
		"UnmarshalRR":               reflect.ValueOf(proto.Unmarshal[proto.PacketRR]),
		"UnmarshalHeartbeat":        reflect.ValueOf(proto.Unmarshal[proto.PacketHeartbeat]),

		"PacketHeartbeat": reflect.ValueOf((*proto.PacketHeartbeat)(nil)),
		"PacketRR":        reflect.ValueOf((*proto.PacketRR)(nil)),
	}
}

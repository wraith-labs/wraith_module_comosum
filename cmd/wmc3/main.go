package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/radio"
	"github.com/gologme/log"
	"github.com/nats-io/nats-server/v2/server"
)

const PRODUCT_NAME = "wmc3"

func main() {
	// Set up logging.
	logger := log.New(os.Stdout, fmt.Sprintf("%s ", PRODUCT_NAME), log.Flags())
	logger.EnableLevelsByNumber(5)
	yggLogger := log.New(os.Stdout, fmt.Sprintf("%s-yggdrasil ", PRODUCT_NAME), log.Flags())

	logger.Infof("starting %s", PRODUCT_NAME)

	// Parse configuration.
	c := MakeConf()

	if c.Debug {
		yggLogger = log.New(os.Stdout, PRODUCT_NAME, log.Flags())
		logger.EnableLevelsByNumber(10)
	}

	// Set up clean exit handler.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT)

	//
	// Create and start NATS server.
	//

	logger.Info("configuring NATS server")

	noExternalListener := c.NatsListener == ""
	systemAccount := server.NewAccount("system")
	opts := &server.Options{
		DontListen:    noExternalListener,
		SystemAccount: systemAccount.Name,
		Accounts: []*server.Account{
			systemAccount,
		},
		Users: []*server.User{
			{
				Username: c.NatsAdminUser,
				Password: c.NatsAdminPass,
				Account:  systemAccount,
			},
		},
	}
	if !noExternalListener {
		listenHost, listenPort, err := net.SplitHostPort(c.NatsListener)
		if err != nil {
			panic(errors.Join(errors.New("failed to parse NATS listen string"), err))
		}

		opts.Host = listenHost
		opts.Port, err = strconv.Atoi(listenPort)
		if err != nil {
			panic(errors.Join(errors.New("failed to parse NATS listen port"), err))
		}
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		panic(errors.Join(errors.New("failed to parse NATS listen port"), err))
	}

	logger.Info("starting NATS server")

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		panic(errors.New("timeout waiting for NATS server to come up"))
	}

	//
	// Create and start the Yggdrasil node.
	//

	logger.Info("starting Yggdrasil node")

	// Set up Yggdrasil.
	n := radio.NewNode(yggLogger)
	n.GenerateConfig(c.YggIdentity, c.YggListeners, c.YggStaticPeers, "none")
	if err := n.Run(); err != nil {
		panic(errors.Join(errors.New("failed to start Yggdrasil node"), err))
	}

	yggaddr, _ := n.Address()

	// Set up userspace network stack to handle Yggdrasil packets.
	s, err := radio.CreateYggdrasilNetstack(n)
	if err != nil {
		panic(err)
	}

	// Set up listener for NATS-over-Ygg.
	listener, _ := s.ListenTCP(&net.TCPAddr{Port: radio.C2_PORT})
	go func() {
		for {
			conn, _ := listener.Accept()
			c, _ := ns.InProcessConn()
			go io.Copy(conn, c)
			go io.Copy(c, conn)
		}
	}()

	logger.Infof("listening on nats://[%s]:45235 (yggdrasil)", yggaddr)
	if !noExternalListener {
		logger.Infof("listening on nats://%s", c.NatsListener)
	}

	// Wait for exit signal.
	<-sigchan

	logger.Info("received exit signal; exiting cleanly")

	// Allow forced exit if cleanup locks up.
	go func() {
		<-sigchan
		logger.Info("received follow-up exit signal; forcing exit")
		os.Exit(1)
	}()

	//
	// Cleanup.
	//

	ns.Shutdown()
}

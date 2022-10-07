package main

type Config struct {
	panelListenAddr        *string
	panelAccessToken       *string
	pineconeId             *string
	logPinecone            *bool
	pineconeInboundTcpAddr *string
	pineconeInboundWebAddr *string
	pineconeDebugEndpoint  *string
	pineconeUseMulticast   *bool
	pineconeStaticPeers    *string
}

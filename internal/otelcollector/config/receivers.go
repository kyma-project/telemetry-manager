package config

type OTLPReceiver struct {
	Protocols ReceiverProtocols `yaml:"protocols,omitempty"`
}

type ReceiverProtocols struct {
	HTTP Endpoint `yaml:"http,omitempty"`
	GRPC Endpoint `yaml:"grpc,omitempty"`
}

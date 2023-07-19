package common

type OTLPReceiverConfig struct {
	Protocols ReceiverProtocols `yaml:"protocols,omitempty"`
}

type ReceiverProtocols struct {
	HTTP EndpointConfig `yaml:"http,omitempty"`
	GRPC EndpointConfig `yaml:"grpc,omitempty"`
}

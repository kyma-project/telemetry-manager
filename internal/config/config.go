package config

type Config struct {
	operateInFipsMode bool
	managerNamespace  string
}

type GlobalConfigProvider interface {
	OperateInFipsMode() bool
	ManagerNamespace() string
}

func NewConfig(operateInFipsMode bool, managerNamespace string) *Config {
	return &Config{
		operateInFipsMode: operateInFipsMode,
		managerNamespace:  managerNamespace,
	}
}

func (c *Config) OperateInFipsMode() bool {
	return c.operateInFipsMode
}

func (c *Config) ManagerNamespace() string {
	return c.managerNamespace
}

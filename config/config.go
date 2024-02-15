package config

type Config struct {
	LogLevel string `default:"warn"`
	Server   ServerConfig
	Gitea    GiteaConfig
	Database DatabaseConfig
	ACME     ACMEConfig
}

type ServerConfig struct {
	Host               string `default:"[::]"`
	Port               uint16 `default:"443"`
	HttpPort           uint16 `default:"80"`
	HttpServerEnabled  bool   `default:"true"`
	MainDomain         string
	RawDomain          string
	PagesBranches      []string
	AllowedCorsDomains []string
	BlacklistedPaths   []string
}

type GiteaConfig struct {
	Root               string
	Token              string
	LFSEnabled         bool   `default:"false"`
	FollowSymlinks     bool   `default:"false"`
	DefaultMimeType    string `default:"application/octet-stream"`
	ForbiddenMimeTypes []string
}

type DatabaseConfig struct {
	Type string `default:"sqlite3"`
	Conn string `default:"certs.sqlite"`
}

type ACMEConfig struct {
	Email             string
	APIEndpoint       string `default:"https://acme-v02.api.letsencrypt.org/directory"`
	AcceptTerms       bool   `default:"false"`
	UseRateLimits     bool   `default:"true"`
	EAB_HMAC          string
	EAB_KID           string
	DNSProvider       string
	AccountConfigFile string `default:"acme-account.json"`
}

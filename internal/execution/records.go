package execution

type Topology string

const (
	TopologyCurrent  Topology = "CURRENT"
	TopologySubagent Topology = "SUBAGENT"
)

type EnvironmentDisposition string

const (
	DispositionInherited      EnvironmentDisposition = "inherited"
	DispositionHostConfigured EnvironmentDisposition = "host-configured"
	DispositionRestricted     EnvironmentDisposition = "restricted"
	DispositionUnknown        EnvironmentDisposition = "unknown"
	DispositionUnavailable    EnvironmentDisposition = "unavailable"
)

type EnvironmentRequirement struct {
	Surface              string                   `json:"surface" toml:"surface"`
	Required             bool                     `json:"required" toml:"required"`
	AcceptedDispositions []EnvironmentDisposition `json:"accepted_dispositions" toml:"accepted_dispositions"`
}

type EnvironmentObservation struct {
	Surface     string                 `json:"surface"`
	Disposition EnvironmentDisposition `json:"disposition"`
	Source      string                 `json:"source"`
	Digest      string                 `json:"digest"`
}
